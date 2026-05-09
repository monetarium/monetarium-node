// Copyright (c) 2026 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
	"github.com/monetarium/monetarium-node/dcrutil"
	"github.com/monetarium/monetarium-node/txscript/stdaddr"
	"github.com/monetarium/monetarium-node/wire"
)

// mainnetSKA1TestParams returns a fresh copy of MainNetParams() with the SKA-1
// EmissionKey replaced by a test pubkey we hold the private half of, so the
// validator can verify a signature we produce. All other SKA-1 fields
// (EmissionAddresses, EmissionAmounts, EmissionHeight, EmissionWindow) come
// from the real mainnet config — this is what makes the test load-bearing.
//
// MainNetParams() constructs a fresh *Params (with fresh map values) on each
// call, so mutating SKACoins[1] does not leak across tests.
func mainnetSKA1TestParams(t *testing.T) (*chaincfg.Params, *secp256k1.PrivateKey) {
	t.Helper()
	params := chaincfg.MainNetParams()

	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("failed to generate test private key: %v", err)
	}
	cfg, ok := params.SKACoins[1]
	if !ok || cfg == nil {
		t.Fatalf("MainNetParams().SKACoins[1] is missing")
	}
	cfg.EmissionKey = priv.PubKey()
	return params, priv
}

// signedSKA1EmissionTx builds an SKA-1 emission transaction with the given
// outputs (addresses + amounts), then signs it with `priv` for validation at
// blockHeight on a fresh chain (where the next expected nonce is 1).
func signedSKA1EmissionTx(t *testing.T, params *chaincfg.Params, priv *secp256k1.PrivateKey,
	addresses []string, amounts []*big.Int, blockHeight int64) *wire.MsgTx {

	t.Helper()
	tx := createTestEmissionTx(t, addresses, amounts, 1, params)

	authAmount := new(big.Int)
	for _, a := range amounts {
		authAmount.Add(authAmount, a)
	}
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: priv.PubKey(),
		CoinType:    1,
		Nonce:       1, // fresh chain: current=0, expected=1
		Amount:      authAmount,
		Height:      blockHeight,
	}
	signEmissionTx(t, tx, auth, priv, params)
	return tx
}

// TestSKA1MainnetEmissionShape pins the historical SKA-1 mainnet emission
// shape (single output to EmissionAddresses[0] paying EmissionAmounts[0])
// against the new per-output-binding validator at
// internal/blockchain/ska_emission.go:435-485. SKA-1 was emitted on mainnet
// at block 4096 with this exact shape; if the validator is ever tightened
// further, this test must continue to pass or full-node reindex will reject
// a previously-valid block.
func TestSKA1MainnetEmissionShape(t *testing.T) {
	t.Run("AcceptCanonicalShape", testSKA1MainnetAcceptCanonicalShape)
	t.Run("AcceptAtWindowEnd", testSKA1MainnetAcceptAtWindowEnd)
	t.Run("RejectExtraOutput", testSKA1MainnetRejectExtraOutput)
	t.Run("RejectWrongAddress", testSKA1MainnetRejectWrongAddress)
	t.Run("RejectWrongAmount", testSKA1MainnetRejectWrongAmount)
}

// testSKA1MainnetAcceptCanonicalShape: a single-output emission paying the
// configured address with the configured full amount, validated at
// EmissionHeight (the height SKA-1 was actually emitted at on mainnet),
// must pass.
func testSKA1MainnetAcceptCanonicalShape(t *testing.T) {
	params, priv := mainnetSKA1TestParams(t)
	cfg := params.SKACoins[1]

	addresses := append([]string(nil), cfg.EmissionAddresses...)
	amounts := append([]*big.Int(nil), cfg.EmissionAmounts...)
	blockHeight := int64(cfg.EmissionHeight)

	tx := signedSKA1EmissionTx(t, params, priv, addresses, amounts, blockHeight)
	chain := createMockChain(t, params)

	if err := ValidateAuthorizedSKAEmissionTransaction(tx, blockHeight, chain, params); err != nil {
		t.Fatalf("canonical SKA-1 emission rejected: %v", err)
	}
}

// testSKA1MainnetAcceptAtWindowEnd: same shape, validated at the last block
// of the emission window. Mainnet SKA-1 used a 30-day window
// (EmissionWindow=4320 from EmissionHeight=4096 → end=8416).
func testSKA1MainnetAcceptAtWindowEnd(t *testing.T) {
	params, priv := mainnetSKA1TestParams(t)
	cfg := params.SKACoins[1]

	addresses := append([]string(nil), cfg.EmissionAddresses...)
	amounts := append([]*big.Int(nil), cfg.EmissionAmounts...)
	blockHeight := int64(cfg.EmissionHeight) + int64(cfg.EmissionWindow)

	tx := signedSKA1EmissionTx(t, params, priv, addresses, amounts, blockHeight)
	chain := createMockChain(t, params)

	if err := ValidateAuthorizedSKAEmissionTransaction(tx, blockHeight, chain, params); err != nil {
		t.Fatalf("SKA-1 emission at window end (height %d) rejected: %v", blockHeight, err)
	}
}

// testSKA1MainnetRejectExtraOutput: the canonical first output plus a second
// output (also paying the emission address) summing to the configured total
// must be rejected by the per-output binding check (output count must equal
// len(EmissionAddresses)).
func testSKA1MainnetRejectExtraOutput(t *testing.T) {
	params, priv := mainnetSKA1TestParams(t)
	cfg := params.SKACoins[1]

	half := new(big.Int).Rsh(cfg.EmissionAmounts[0], 1)
	other := new(big.Int).Sub(cfg.EmissionAmounts[0], half)
	addresses := []string{cfg.EmissionAddresses[0], cfg.EmissionAddresses[0]}
	amounts := []*big.Int{half, other}
	blockHeight := int64(cfg.EmissionHeight)

	tx := signedSKA1EmissionTx(t, params, priv, addresses, amounts, blockHeight)
	chain := createMockChain(t, params)

	if err := ValidateAuthorizedSKAEmissionTransaction(tx, blockHeight, chain, params); err == nil {
		t.Fatalf("extra-output emission accepted; expected rejection")
	}
}

// testSKA1MainnetRejectWrongAddress: a single-output tx paying a different
// (still decodable) mainnet address must be rejected by the per-output
// PkScript comparison.
func testSKA1MainnetRejectWrongAddress(t *testing.T) {
	params, priv := mainnetSKA1TestParams(t)
	cfg := params.SKACoins[1]

	otherPriv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate alt key: %v", err)
	}
	pkBytes := otherPriv.PubKey().SerializeCompressed()
	otherAddr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(
		dcrutil.Hash160(pkBytes), params)
	if err != nil {
		t.Fatalf("derive alt address: %v", err)
	}
	if otherAddr.String() == cfg.EmissionAddresses[0] {
		t.Fatalf("alt address collided with emission address")
	}

	addresses := []string{otherAddr.String()}
	amounts := []*big.Int{new(big.Int).Set(cfg.EmissionAmounts[0])}
	blockHeight := int64(cfg.EmissionHeight)

	tx := signedSKA1EmissionTx(t, params, priv, addresses, amounts, blockHeight)
	chain := createMockChain(t, params)

	if err := ValidateAuthorizedSKAEmissionTransaction(tx, blockHeight, chain, params); err == nil {
		t.Fatalf("wrong-address emission accepted; expected rejection")
	}
}

// testSKA1MainnetRejectWrongAmount: a single-output tx paying the correct
// address but EmissionAmounts[0]-1 must be rejected by the per-output amount
// comparison.
func testSKA1MainnetRejectWrongAmount(t *testing.T) {
	params, priv := mainnetSKA1TestParams(t)
	cfg := params.SKACoins[1]

	addresses := append([]string(nil), cfg.EmissionAddresses...)
	short := new(big.Int).Sub(cfg.EmissionAmounts[0], big.NewInt(1))
	amounts := []*big.Int{short}
	blockHeight := int64(cfg.EmissionHeight)

	tx := signedSKA1EmissionTx(t, params, priv, addresses, amounts, blockHeight)
	chain := createMockChain(t, params)

	if err := ValidateAuthorizedSKAEmissionTransaction(tx, blockHeight, chain, params); err == nil {
		t.Fatalf("wrong-amount emission accepted; expected rejection")
	}
}
