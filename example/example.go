package main

import (
	cryptoecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"
	ethereumhexutil "github.com/ethereum/go-ethereum/common/hexutil"
	ethereumcrypto "github.com/ethereum/go-ethereum/crypto"
	ethereumsecp256k1 "github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/fxamacker/cbor/v2"
	"github.com/w3-key/mps-lean/pkg/ecdsa"
	"github.com/w3-key/mps-lean/pkg/math/curve"
	"github.com/w3-key/mps-lean/pkg/party"
	"github.com/w3-key/mps-lean/pkg/pool"
	"github.com/w3-key/mps-lean/pkg/protocol"
	"github.com/w3-key/mps-lean/pkg/test"
	"github.com/w3-key/mps-lean/protocols/cmp"
	"github.com/w3-key/mps-lean/protocols/cmp/sign"
	"github.com/w3-key/mps-lean/protocols/example"
)

//type SignatureParts struct {
//	GroupDelta    curve.Scalar
//	GroupBigDelta curve.Point
//	GroupKShare   curve.Scalar
//	GroupBigR     curve.Point
//	GroupChiShare curve.Scalar
//	Group         curve.Curve
//}

var signaturePartsArray = []*sign.SignatureParts{}
var signaturesArray = []string{}

func XOR(id party.ID, ids party.IDSlice, n *test.Network) error {
	h, err := protocol.NewMultiHandler(example.StartXOR(id, ids), nil)
	if err != nil {
		return err
	}
	test.HandlerLoop(id, h, n)
	_, err = h.Result()
	if err != nil {
		return err
	}
	return nil
}

func CMPKeygen(id party.ID, ids party.IDSlice, threshold int, n *test.Network, pl *pool.Pool) (*cmp.Config, error) {
	h, err := protocol.NewMultiHandler(cmp.Keygen(curve.Secp256k1{}, id, ids, threshold, pl), nil)
	if err != nil {
		return nil, err
	}

	test.HandlerLoop(id, h, n)
	r, err := h.Result()
	if err != nil {
		return nil, err
	}

	config := r.(*cmp.Config)

	return config, nil
}

func CMPRefresh(c *cmp.Config, n *test.Network, pl *pool.Pool) (*cmp.Config, error) {
	hRefresh, err := protocol.NewMultiHandler(cmp.Refresh(c, pl), nil)
	if err != nil {
		return nil, err
	}
	test.HandlerLoop(c.ID, hRefresh, n)

	r, err := hRefresh.Result()
	if err != nil {
		return nil, err
	}

	return r.(*cmp.Config), nil
}

func SingleSign(specialconfig sign.SignatureParts, message []byte) (signresult curve.Scalar) {
	fmt.Println("SINGLESIGN")
	//spew.Dump(specialconfig)
	group := specialconfig.Group
	//Delta = specialconfig.GroupDelta
	//BigDelta = specialconfig.GroupBigDelta
	//GammaShare, BigGammaShare := sample.ScalarPointPair(rand.Reader, group)
	//GShare := specialconfig.MakeInt(GammaShare)
	KShare := specialconfig.GroupKShare
	//KShareInt := curve.MakeInt(KShare)
	//Gamma := group.NewPoint()
	//Gamma = Gamma.Add(BigGammaShare)
	//deltaComputed := Delta.ActOnBase()
	//if !deltaComputed.Equal(BigDelta) {
	//	fmt.Println("computed Δ is inconsistent with [δ]G")
	//}
	//deltaInv := group.NewScalar().Set(Delta).Invert() // δ⁻¹

	BigR := specialconfig.GroupBigR // R = [δ⁻¹] Γ
	R := BigR.XScalar()             // r = R|ₓ
	km := curve.FromHash(group, message)
	km.Mul(KShare)
	SigmaShare := group.NewScalar().Set(R).Mul(specialconfig.GroupChiShare).Add(km)
	return SigmaShare
}

func CombineSignatures(SigmaShares curve.Scalar, specialConfig sign.SignatureParts) (signature ecdsa.Signature) {
	//for _, j := range SigmaShares.length{
	//	Sigma.Add(SigmaShares[j])
	//}
		combinedSig := ecdsa.Signature{
			R: specialConfig.GroupBigR,
			S: SigmaShares,
		}
		return combinedSig;
}

func CMPSignGetExtraInfo(c *cmp.Config, m []byte, signers party.IDSlice, n *test.Network, pl *pool.Pool, justinfo bool) (sign.SignatureParts, error) {
	h, _ := protocol.NewMultiHandler(cmp.Sign(c, signers, m, pl, justinfo), nil)
	test.HandlerLoop(c.ID, h, n)
	signResult, _ := h.Result()
	signaturestuff := signResult.(sign.SignatureParts)
	//spew.Dump(signaturestuff)
	messageToSign := ethereumcrypto.Keccak256([]byte("Hi"))
	blehsign := SingleSign(signaturestuff,messageToSign)

	combined := CombineSignatures(blehsign, signaturestuff)
	spew.Dump(combined)
	//sig, err := ethereumhexutil.Decode("0xd8d963bf1fd8e09cc7a55d1f5f39c762036017d662b87e58403752078952be5e34a5dbe67b18b2a9fd46c96866a3c0118d092df8219d0f69034dd8949ed8c34a1c")

	// println(len(rb), len(sb), len(sig), len(m), recoverId, "rb len")

	//m = []byte("0xc019d8a5f1cbf05267e281484f3ddc2394a6b5eacc14e9d210039cf34d8391fc")
	//sig[64] = sig[64] - 27
	//
	//// println(hex.EncodeToString(sig), "sign")
	//if ss, err := secp256k1.RecoverPubkey(m, sig); err != nil {
	//	return signatureParts, err
	//} else {
	//	// bs, _ := c.PublicPoint().MarshalBinary()
	//	x, y := elliptic.Unmarshal(secp256k1.S256(), ss)
	//	pk := cryptoecdsa.PublicKey{Curve: secp256k1.S256(), X: x, Y: y}
	//
	//	pk2 := c.PublicPoint().ToAddress().Hex()
	//	println(ethereumcrypto.PubkeyToAddress(pk).Hex(), "public key", pk2)
	//}
	//
	//if !signature.Verify(c.PublicPoint(), m) {
	//	return signatureParts, errors.New("failed to verify cmp signature")
	//}

	//spew.Dump(signResult.(*sign.signatureParts))

	//signaturePartsVar := signatureParts{
	//	&signResult.GroupDelta,
	//	&signResult.GroupBigDelta,
	//	&signResult.GroupKShare,
	//	&signResult.GroupBigR,
	//	&signResult.GroupChiShare,
	//}

	return signaturestuff, nil
}

func CMPSign(c *cmp.Config, m []byte, signers party.IDSlice, n *test.Network, pl *pool.Pool, justinfo bool) error {
	h, err := protocol.NewMultiHandler(cmp.Sign(c, signers, m, pl, justinfo), nil)
	if err != nil {
		return err
	}
	test.HandlerLoop(c.ID, h, n)

	signResult, err := h.Result()
	if err != nil {
		return err
	}
	signature := signResult.(*ecdsa.Signature)

	if err != nil {
		return err
	}

	sig, err := ethereumhexutil.Decode("0xd8d963bf1fd8e09cc7a55d1f5f39c762036017d662b87e58403752078952be5e34a5dbe67b18b2a9fd46c96866a3c0118d092df8219d0f69034dd8949ed8c34a1c")

	if err != nil {
		return err
	}
	// println(len(rb), len(sb), len(sig), len(m), recoverId, "rb len")

	m = []byte("0xc019d8a5f1cbf05267e281484f3ddc2394a6b5eacc14e9d210039cf34d8391fc")
	sig[64] = sig[64] - 27

	// println(hex.EncodeToString(sig), "sign")
	if ss, err := ethereumsecp256k1.RecoverPubkey(m, sig); err != nil {
		return err
	} else {
		// bs, _ := c.PublicPoint().MarshalBinary()
		x, y := elliptic.Unmarshal(ethereumsecp256k1.S256(), ss)
		pk := cryptoecdsa.PublicKey{Curve: ethereumsecp256k1.S256(), X: x, Y: y}

		pk2 := c.PublicPoint().ToAddress().Hex()
		println(ethereumcrypto.PubkeyToAddress(pk).Hex(), "public key", pk2)
	}

	if !signature.Verify(c.PublicPoint(), m) {
		return errors.New("failed to verify cmp signature")
	}
	return nil
}

func All(id party.ID, ids party.IDSlice, threshold int, message []byte, n *test.Network, wg *sync.WaitGroup, pl *pool.Pool) error {
	defer wg.Done()
	err := XOR(id, ids, n)
	if err != nil {
		return err
	}
	keygenConfig, err := CMPKeygen(id, ids, threshold, n, pl)
	if err != nil {
		return err
	}
	fmt.Println(keygenConfig.PublicPoint().ToAddress())
	refreshConfig, err := CMPRefresh(keygenConfig, n, pl)
	fmt.Println(refreshConfig.PublicPoint().ToAddress())
	signers := ids[:threshold+1]
	if !signers.Contains(id) {
		n.Quit(id)
		return nil
	}

	result, _ := CMPSignGetExtraInfo(refreshConfig, message, signers, n, pl, true)
	marshalSignData, _ := cbor.Marshal(result)
	spew.Dump(marshalSignData)
	//var group = curve.Secp256k1{}

	//unmarshalledConfig := sign.EmptyConfig(group)
	//spew.Dump(unmarshalledConfig)
	//err = cbor.Unmarshal(marshalSignData, unmarshalledConfig)
	//spew.Dump(unmarshalledConfig)
	//messageToSign := ethereumcrypto.Keccak256([]byte("Hi"))
	//signitup := SingleSign(unmarshalledConfig, messageToSign)

	//spew.Dump(signitup)

	// CMP SIGN
	//err = CmpSign(refreshConfig, message, signers, n, pl)
	//if err != nil {
	//	return err
	//}

	return nil
}

func main() {
	ids := party.IDSlice{"a", "b"}
	threshold := 1
	messageToSign := ethereumcrypto.Keccak256([]byte("Hi"))
	net := test.NewNetwork(ids)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id party.ID) {
			pl := pool.NewPool(10)
			defer pl.TearDown()
			if err := All(id, ids, threshold, messageToSign, net, &wg, pl); err != nil {
				fmt.Println(err)
			}
		}(id)
	}
	wg.Wait()
}
