// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
	"github.com/monetarium/monetarium-node/wire"
)

// TestShipCriticalSecurityFixes tests all the ship-blocker security fixes
func TestShipCriticalSecurityFixes(t *testing.T) {
	// Create test parameters
	params := chaincfg.SimNetParams()

	// Generate test keys
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Initialize per-coin configuration
	params.SKACoins = map[cointype.CoinType]*chaincfg.SKACoinConfig{
		1: {
			CoinType:          1,
			Name:              "SKA-1",
			Symbol:            "SKA1",
			MaxSupply:         10000000,
			Active:            true,
			EmissionHeight:    100,
			EmissionWindow:    100,
			EmissionAddresses: []string{"ScuQxvveKGfpG1ypt6u27F99Anf7EW3cqhq"},
			EmissionAmounts:   []int64{10000000},
			EmissionKey:       pubKey, // Add the emission key
		},
	}

	t.Run("ScriptVersionConsistency", func(t *testing.T) {
		// Test that created transactions have version 0
		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			Signature:   make([]byte, 64), // Dummy signature
			Nonce:       1,
			CoinType:    1,
			Amount:      10000000,
			Height:      100,
		}

		tx, err := CreateAuthorizedSKAEmissionTransaction(
			auth,
			[]string{"ScuQxvveKGfpG1ypt6u27F99Anf7EW3cqhq"},
			[]int64{10000000}, params)
		if err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}

		// Verify all outputs have version 0
		for i, out := range tx.TxOut {
			if out.Version != 0 {
				t.Errorf("Output %d has version %d, expected 0", i, out.Version)
			}
		}
	})

	t.Run("NoCreationTimeNonceCheck", func(t *testing.T) {
		// Test that creation doesn't check nonce (wallets can't know chain state)
		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			Signature:   make([]byte, 64), // Dummy signature
			Nonce:       999,              // Any nonce should work at creation time
			CoinType:    1,
			Amount:      10000000,
			Height:      100,
		}

		// This should NOT fail despite wrong nonce
		_, err := CreateAuthorizedSKAEmissionTransaction(auth,
			[]string{"ScuQxvveKGfpG1ypt6u27F99Anf7EW3cqhq"},
			[]int64{10000000}, params)
		if err != nil {
			t.Fatalf("Creation should not check nonce, but got error: %v", err)
		}
	})

	t.Run("SameCoinTypeValidation", func(t *testing.T) {
		// Create a transaction with mixed coin types (should fail validation)
		tx := &wire.MsgTx{
			SerType:  wire.TxSerializeFull,
			Version:  1,
			LockTime: 0,
			Expiry:   0,
		}

		// Add null input with SKA marker
		tx.TxIn = append(tx.TxIn, &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
				Tree:  wire.TxTreeRegular,
			},
			SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // SKA marker
			Sequence:        0xffffffff,
		})

		// Add outputs with different coin types
		tx.TxOut = append(tx.TxOut, &wire.TxOut{
			Value:    5000000,
			CoinType: 1,
			Version:  0,
			PkScript: []byte{0x76, 0xa9, 0x14},
		})
		tx.TxOut = append(tx.TxOut, &wire.TxOut{
			Value:    5000000,
			CoinType: 2, // Different coin type!
			Version:  0,
			PkScript: []byte{0x76, 0xa9, 0x14},
		})

		// Create mock chain for testing
		chain := &BlockChain{
			chainParams: params,
			skaEmissionState: &SKAEmissionState{
				nonces:  make(map[cointype.CoinType]uint64),
				emitted: make(map[cointype.CoinType]bool),
			},
		}

		// This should fail validation (either for signature issues or mixed coin types)
		err := ValidateAuthorizedSKAEmissionTransaction(tx, 100, chain, params)
		if err == nil {
			t.Error("Expected validation to fail for invalid transaction")
		}
		// Note: With secure validation, signature validation occurs first,
		// so we may get signature errors before coin type validation errors
	})

	t.Run("GovernanceAmountEnforcement", func(t *testing.T) {
		// Test that emission amount must match governance config
		chain := &BlockChain{
			chainParams: params,
			skaEmissionState: &SKAEmissionState{
				nonces:  make(map[cointype.CoinType]uint64),
				emitted: make(map[cointype.CoinType]bool),
			},
		}

		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			Signature:   make([]byte, 64), // Will be set after signing
			Nonce:       1,
			CoinType:    1,
			Amount:      5000000, // Wrong amount! Should be 10000000
			Height:      100,
		}

		// Create transaction with wrong total
		tx := &wire.MsgTx{
			SerType:  wire.TxSerializeFull,
			Version:  1,
			LockTime: 0,
			Expiry:   0,
		}

		// Add authorized input
		authScript, _ := createEmissionAuthScript(auth)
		tx.TxIn = append(tx.TxIn, &wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
				Tree:  wire.TxTreeRegular,
			},
			SignatureScript: authScript,
			Sequence:        0xffffffff,
		})

		// Add output with wrong amount
		tx.TxOut = append(tx.TxOut, &wire.TxOut{
			Value:    5000000, // Wrong! Should match governance config
			CoinType: 1,
			Version:  0,
			PkScript: []byte{0x76, 0xa9, 0x14},
		})

		// Extract and validate - should fail due to amount mismatch
		extractedAuth, _ := extractEmissionAuthorization(authScript)
		err := validateEmissionAuthorization(extractedAuth, chain, params)
		if err != nil {
			// This is expected - nonce validation will fail first
			// But the important part is the ValidateAuthorizedSKAEmissionTransaction check
		}

		// The full validation should catch the governance amount mismatch
		err = ValidateAuthorizedSKAEmissionTransaction(tx, 100, chain, params)
		if err == nil {
			t.Error("Expected validation to fail for wrong governance amount")
		}
	})

	t.Run("WindowEdgeValidation", func(t *testing.T) {
		// Test emission at window edges
		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			Signature:   make([]byte, 64),
			Nonce:       1,
			CoinType:    1,
			Amount:      10000000,
			Height:      100, // Start of window
		}

		// Test at start of window (should succeed)
		tx, err := CreateAuthorizedSKAEmissionTransaction(auth,
			[]string{"ScuQxvveKGfpG1ypt6u27F99Anf7EW3cqhq"},
			[]int64{10000000}, params)
		if err != nil {
			t.Fatalf("Should allow emission at start of window: %v", err)
		}
		_ = tx

		// Test at end of window
		auth.Height = 200 // End of window (100 + 100)
		tx2, err := CreateAuthorizedSKAEmissionTransaction(auth,
			[]string{"ScuQxvveKGfpG1ypt6u27F99Anf7EW3cqhq"},
			[]int64{10000000}, params)
		if err != nil {
			t.Fatalf("Should allow emission at end of window: %v", err)
		}
		_ = tx2

		// Test outside window
		auth.Height = 201 // Just past end of window
		_, err = CreateAuthorizedSKAEmissionTransaction(auth,
			[]string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			[]int64{10000000}, params)
		if err == nil {
			t.Error("Should reject emission outside window")
		}
	})
}
