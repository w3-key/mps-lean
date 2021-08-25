package ot

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"

	"github.com/cronokirby/safenum"
	"github.com/taurusgroup/multi-party-sig/internal/params"
	"github.com/taurusgroup/multi-party-sig/pkg/hash"
	"github.com/taurusgroup/multi-party-sig/pkg/math/curve"
	"github.com/taurusgroup/multi-party-sig/pkg/math/sample"
	zksch "github.com/taurusgroup/multi-party-sig/pkg/zk/sch"
)

// RandomOTSetupSendMessage is the message generated by the sender of the OT.
type RandomOTSetupSendMessage struct {
	// A public key used for subsequent random OTs.
	_B curve.Point
	// A proof of the discrete log of this public key.
	_BProof *zksch.Proof
}

// RandomOTSetupSendResult is the result that should be saved for the sender.
//
// This result can be used for multiple random OTs later.
type RandomOTSetupSendResult struct {
	// A secret key used for subsequent random OTs.
	b curve.Scalar
	// The matching public key.
	_B curve.Point
}

// RandomOTSetupSend runs the Sender's part of the setup protocol for Random OT.
//
// The hash should be used to tie the execution of the protocol to the ambient context,
// if that's desired.
//
// This setup can be done once and then used for multiple executions.
func RandomOTSetupSend(hash *hash.Hash, group curve.Curve) (*RandomOTSetupSendMessage, *RandomOTSetupSendResult) {
	b := sample.Scalar(rand.Reader, group)
	B := b.ActOnBase()
	BProof := zksch.NewProof(hash, B, b)
	return &RandomOTSetupSendMessage{_B: B, _BProof: BProof}, &RandomOTSetupSendResult{_B: B, b: b}
}

// RandomOTSetupReceiveResult is the result that should be saved for the receiver.
type RandomOTSetupReceiveResult struct {
	// The public key for the sender, used for subsequent random OTs.
	_B curve.Point
}

// RandomOTSetupReceive runs the Receiver's part of the setup protocol for Random OT.
//
// The hash should be used to tie the execution of the protocol to the ambient context,
// if that's desired.
//
// This setup can be done once and then used for multiple executions.
func RandomOTSetupReceive(hash *hash.Hash, msg *RandomOTSetupSendMessage) (*RandomOTSetupReceiveResult, error) {
	if !msg._BProof.Verify(hash, msg._B) {
		return nil, fmt.Errorf("RandomOTSetupReceive: Schnorr proof failed to verify")
	}
	return &RandomOTSetupReceiveResult{_B: msg._B}, nil
}

// RandomOTReceiver contains the state needed for a single execution of a Random OT.
//
// This should be created from a saved setup, for each execution.
type RandomOTReceiever struct {
	// After setup
	hash  *hash.Hash
	group curve.Curve
	// Which random message we want to receive.
	choice safenum.Choice
	// The public key of the sender.
	_B curve.Point
	// After Round1

	// The random message we've received.
	randChoice []byte
	// After Round2

	// The challenge sent to use by the sender.
	receivedChallenge []byte
	// H(H(randChoice)), used to avoid redundant calculations.
	hh_randChoice []byte
}

// NewRandomOTReceiver sets up the receiver's state for a single Random OT.
//
// If multiple executions are done with the same setup, it's crucial that the hash is
// initialized with some kind of nonce.
//
// choice indicates which of the two random messages should be received.
func NewRandomOTReceiver(hash *hash.Hash, choice safenum.Choice, result *RandomOTSetupReceiveResult) *RandomOTReceiever {
	return &RandomOTReceiever{hash: hash, group: result._B.Curve(), choice: choice, _B: result._B}
}

// RandomOTReceiveRound1Message is the first message sent by the receiver in a Random OT.
type RandomOTReceiveRound1Message struct {
	_A curve.Point
}

// Round1 executes the receiver's side of round 1 of a Random OT.
//
// This is the starting point for a Random OT.
func (r *RandomOTReceiever) Round1() *RandomOTReceiveRound1Message {
	// We sample a <- Z_q, and then compute
	//   A = a * G + w * B
	//   randChoice = H(a * B)
	a := sample.Scalar(rand.Reader, r.group)
	A := a.ActOnBase()
	one := new(safenum.Nat).SetUint64(1)
	choiceScalar := r.group.NewScalar().SetNat(new(safenum.Nat).CondAssign(r.choice, one))
	A = A.Add(choiceScalar.Act(r._B))

	r.randChoice = make([]byte, params.SecBytes)
	_ = r.hash.WriteAny(a.Act(r._B))
	_, _ = r.hash.Digest().Read(r.randChoice)

	return &RandomOTReceiveRound1Message{_A: A}
}

// RandomOTReceiveRound2Message is the second message sent by the receiver in a Random OT.
type RandomOTReceiveRound2Message struct {
	// A response to the challenge submitted by the sender.
	response []byte
}

// Round2 executes the receiver's side of round 2 of a Random OT.
func (r *RandomOTReceiever) Round2(msg *RandomOTSendRound1Message) *RandomOTReceiveRound2Message {
	r.receivedChallenge = msg.challenge
	// response = H(H(randW)) ^ (w * challenge).
	response := make([]byte, len(msg.challenge))

	H := hash.New()
	_ = H.WriteAny(r.randChoice)
	_, _ = H.Digest().Read(response)
	H = hash.New()
	_ = H.WriteAny(response)
	_, _ = H.Digest().Read(response)

	r.hh_randChoice = make([]byte, len(response))
	copy(r.hh_randChoice, response)

	mask := -byte(r.choice)
	for i := 0; i < len(msg.challenge); i++ {
		response[i] ^= mask & msg.challenge[i]
	}

	return &RandomOTReceiveRound2Message{response: response}
}

// Round3 finalizes the result for the receiver, performing verification.
//
// The random choice is returned as the first argument, upon success.
func (r *RandomOTReceiever) Round3(msg *RandomOTSendRound2Message) ([]byte, error) {
	h_decommit0 := make([]byte, len(r.receivedChallenge))
	H := hash.New()
	_ = H.WriteAny(msg.decommit0)
	_, _ = H.Digest().Read(h_decommit0)

	h_decommit1 := make([]byte, len(r.receivedChallenge))
	H = hash.New()
	_ = H.WriteAny(msg.decommit1)
	_, _ = H.Digest().Read(h_decommit1)

	actualChallenge := make([]byte, len(r.receivedChallenge))
	for i := 0; i < len(r.receivedChallenge); i++ {
		actualChallenge[i] = h_decommit0[i] ^ h_decommit1[i]
	}

	if subtle.ConstantTimeCompare(r.receivedChallenge, actualChallenge) != 1 {
		return nil, fmt.Errorf("RandomOTReceive Round 3: incorrect decommitment")
	}

	// Assign the decommitment hash to the one matching our own choice
	h_decommitChoice := h_decommit0
	mask := -byte(r.choice)
	for i := 0; i < len(r.receivedChallenge); i++ {
		h_decommitChoice[i] ^= (mask & (h_decommitChoice[i] ^ h_decommit1[i]))
	}
	if subtle.ConstantTimeCompare(h_decommitChoice, r.hh_randChoice) != 1 {
		return nil, fmt.Errorf("RandomOTReceive Round 3: incorrect decommitment")
	}

	return r.randChoice, nil
}

// RandomOTSender holds the state needed for a single execution of a Random OT.
//
// This should be created from a saved setup, for each execution.
type RandomOTSender struct {
	// After setup
	hash *hash.Hash
	b    curve.Scalar
	_B   curve.Point
	// After round 1
	rand0 []byte
	rand1 []byte

	decommit0 []byte
	decommit1 []byte

	h_decommit0 []byte
}

// NewRandomOTSender sets up the receiver's state for a single Random OT.
//
// If multiple executions are done with the same setup, it's crucial that the hash is
// initialized with some kind of nonce.
func NewRandomOTSender(hash *hash.Hash, result *RandomOTSetupSendResult) *RandomOTSender {
	return &RandomOTSender{hash: hash, b: result.b, _B: result._B}
}

// RandomOTSendRound1Message is the message sent by the sender in round 1.
type RandomOTSendRound1Message struct {
	challenge []byte
}

// Round1 executes the sender's side of round 1 for a Random OT.
func (r *RandomOTSender) Round1(msg *RandomOTReceiveRound1Message) *RandomOTSendRound1Message {
	// We can compute the two random pads:
	//    rand0 = H(b * A)
	//    rand1 = H(b * (A - B))
	r.rand0 = make([]byte, params.SecBytes)
	H := r.hash.Clone()
	_ = H.WriteAny(r.b.Act(msg._A))
	_, _ = H.Digest().Read(r.rand0)

	r.rand1 = make([]byte, params.SecBytes)
	H = r.hash.Clone()
	_ = H.WriteAny(r.b.Act(msg._A.Sub(r._B)))
	_, _ = H.Digest().Read(r.rand1)

	// Compute the challenge:
	//   H(H(rand0)) ^ H(H(rand1))
	r.decommit0 = make([]byte, params.SecBytes)
	H = hash.New()
	_ = H.WriteAny(r.rand0)
	_, _ = H.Digest().Read(r.decommit0)

	r.decommit1 = make([]byte, params.SecBytes)
	H = hash.New()
	_ = H.WriteAny(r.rand1)
	_, _ = H.Digest().Read(r.decommit1)

	r.h_decommit0 = make([]byte, params.SecBytes)
	H = hash.New()
	_ = H.WriteAny(r.decommit0)
	_, _ = H.Digest().Read(r.h_decommit0)

	challenge := make([]byte, params.SecBytes)
	H = hash.New()
	_ = H.WriteAny(r.decommit1)
	_, _ = H.Digest().Read(challenge)

	for i := 0; i < len(challenge) && i < len(r.h_decommit0); i++ {
		challenge[i] ^= r.h_decommit0[i]
	}

	return &RandomOTSendRound1Message{challenge: challenge}
}

// RandomOTSendRound2Message is the message sent by the sender in round 2 of a Random OT.
type RandomOTSendRound2Message struct {
	decommit0 []byte
	decommit1 []byte
}

// RandomOTSendResult is the result for a sender in a Random OT.
//
// We have two random results with a symmetric security parameter's worth of bits each.
type RandomOTSendResult struct {
	// Rand0 is the first random message.
	Rand0 []byte
	// Rand1 is the second random message.
	Rand1 []byte
}

// Round2 executes the sender's side of round 2 in a Random OT.
func (r *RandomOTSender) Round2(msg *RandomOTReceiveRound2Message) (*RandomOTSendRound2Message, *RandomOTSendResult, error) {
	if subtle.ConstantTimeCompare(msg.response, r.h_decommit0) != 1 {
		return nil, nil, fmt.Errorf("RandomOTSender Round2: invalid response")
	}

	return &RandomOTSendRound2Message{decommit0: r.decommit0, decommit1: r.decommit1}, &RandomOTSendResult{Rand0: r.rand0, Rand1: r.rand1}, nil
}
