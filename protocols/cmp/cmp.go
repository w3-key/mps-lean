package cmp

import (
	"github.com/w3-key/mps-lean/pkg/math/curve"
	"github.com/w3-key/mps-lean/pkg/party"
	"github.com/w3-key/mps-lean/pkg/pool"
	"github.com/w3-key/mps-lean/pkg/protocol"
	"github.com/w3-key/mps-lean/pkg/round"
	"github.com/w3-key/mps-lean/protocols/cmp/config"
	"github.com/w3-key/mps-lean/protocols/cmp/keygen"
	"github.com/w3-key/mps-lean/protocols/cmp/sign"
)

// Config represents the stored state of a party who participated in a successful `Keygen` protocol.
// It contains secret key material and should be safely stored.
type Config = config.Config

// EmptyConfig creates an empty Config with a fixed group, ready for unmarshalling.
//
// This needs to be used for unmarshalling, otherwise the points on the curve can't
// be decoded.
func EmptyConfig(group curve.Curve) *Config {
	return &Config{
		Group: group,
	}
}

// Keygen generates a new shared ECDSA key over the curve defined by `group`. After a successful execution,
// all participants posses a unique share of this key, as well as auxiliary parameters required during signing.
//
// For better performance, a `pool.Pool` can be provided in order to parallelize certain steps of the protocol.
// Returns *cmp.Config if successful.
func Keygen(group curve.Curve, selfID party.ID, participants []party.ID, threshold int, pl *pool.Pool) protocol.StartFunc {
	info := round.Info{
		ProtocolID:       "cmp/keygen-threshold",
		FinalRoundNumber: keygen.Rounds,
		SelfID:           selfID,
		PartyIDs:         participants,
		Threshold:        threshold,
		Group:            group,
	}
	return keygen.Start(info, pl, nil)
}

// Refresh allows the parties to refresh all existing cryptographic keys from a previously generated Config.
// The group's ECDSA public key remains the same, but any previous shares are rendered useless.
// Returns *cmp.Config if successful.
func Refresh(config *Config, pl *pool.Pool) protocol.StartFunc {
	info := round.Info{
		ProtocolID:       "cmp/refresh-threshold",
		FinalRoundNumber: keygen.Rounds,
		SelfID:           config.ID,
		PartyIDs:         config.PartyIDs(),
		Threshold:        config.Threshold,
		Group:            config.Group,
	}
	return keygen.Start(info, pl, config)
}

// Sign generates an ECDSA signature for `messageHash` among the given `signers`.
// Returns *ecdsa.Signature if successful.
func Sign(config *Config, signers []party.ID, messageHash []byte, pl *pool.Pool, forkeys bool) protocol.StartFunc {
	return sign.StartSign(config, signers, messageHash, pl, forkeys)
}
