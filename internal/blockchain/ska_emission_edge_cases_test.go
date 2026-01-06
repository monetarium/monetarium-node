// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
)

// TestSKAEmissionBasicValidation tests basic SKA emission validation.
func TestSKAEmissionBasicValidation(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Get SKA coin type 1 for consistent testing
	config := params.SKACoins[1]
	if config == nil {
		t.Fatal("SKA coin type 1 not found in simnet params")
	}

	// Create mock chain for testing
	chain := &BlockChain{
		chainParams: params,
		skaEmissionState: &SKAEmissionState{
			nonces:  make(map[cointype.CoinType]uint64),
			emitted: make(map[cointype.CoinType]bool),
		},
	}

	t.Run("ValidEmission", func(t *testing.T) {
		// Calculate expected total emission amount from config
		var expectedEmissionAmount int64
		for _, amount := range config.EmissionAmounts {
			expectedEmissionAmount += amount
		}

		// Generate test key and configure params
		privKey, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("Failed to generate private key: %v", err)
		}
		pubKey := privKey.PubKey()
		config.EmissionKey = pubKey

		// Create a valid emission transaction with proper authorization
		addresses := []string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
		amounts := []int64{expectedEmissionAmount}
		tx := createTestEmissionTx(t, addresses, amounts, cointype.CoinType(1), params)

		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			CoinType:    cointype.CoinType(1),
			Nonce:       1,
			Amount:      expectedEmissionAmount,
			Height:      int64(config.EmissionHeight),
		}
		signEmissionTx(t, tx, auth, privKey, params)

		// Test at the correct emission height using secure validation
		err = ValidateAuthorizedSKAEmissionTransaction(tx, int64(config.EmissionHeight), chain, params)
		if err != nil {
			t.Errorf("Valid emission transaction should pass: %v", err)
		}
	})

	t.Run("InvalidAmount", func(t *testing.T) {
		// Calculate expected total emission amount from config
		var expectedEmissionAmount int64
		for _, amount := range config.EmissionAmounts {
			expectedEmissionAmount += amount
		}

		// Create emission with wrong amount by using CreateAuthorizedSKAEmissionTransaction
		// which validates the amount against authorization
		emissionKey := params.GetSKAEmissionKey(cointype.CoinType(1))
		if emissionKey == nil {
			t.Fatal("No emission key configured for SKA-1")
		}
		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: emissionKey,
			Signature:   make([]byte, 64), // Dummy signature
			Nonce:       1,
			CoinType:    cointype.CoinType(1),
			Amount:      expectedEmissionAmount, // Auth for correct amount
			Height:      10,
		}
		_, err := CreateAuthorizedSKAEmissionTransaction(
			auth,
			[]string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			[]int64{expectedEmissionAmount + 1},
			params,
		)
		if err == nil {
			t.Error("Emission with wrong amount should fail")
		}
	})

	t.Run("InvalidHeight", func(t *testing.T) {
		// Calculate expected total emission amount from config
		var expectedEmissionAmount int64
		for _, amount := range config.EmissionAmounts {
			expectedEmissionAmount += amount
		}

		// Generate test key and configure params
		privKey, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("Failed to generate private key: %v", err)
		}
		pubKey := privKey.PubKey()
		config.EmissionKey = pubKey

		// Create valid emission at wrong height
		addresses := []string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
		amounts := []int64{expectedEmissionAmount}
		tx := createTestEmissionTx(t, addresses, amounts, cointype.CoinType(1), params)

		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			CoinType:    cointype.CoinType(1),
			Nonce:       1,
			Amount:      expectedEmissionAmount,
			Height:      int64(config.EmissionHeight),
		}
		signEmissionTx(t, tx, auth, privKey, params)

		err = ValidateAuthorizedSKAEmissionTransaction(tx, int64(config.EmissionHeight)+1000, chain, params)
		if err == nil {
			t.Error("Emission at wrong height should fail")
		}
	})

	t.Run("VAROutput", func(t *testing.T) {
		// Calculate expected total emission amount from config
		var expectedEmissionAmount int64
		for _, amount := range config.EmissionAmounts {
			expectedEmissionAmount += amount
		}

		// Generate test key and configure params
		privKey, err := secp256k1.GeneratePrivateKey()
		if err != nil {
			t.Fatalf("Failed to generate private key: %v", err)
		}
		pubKey := privKey.PubKey()
		config.EmissionKey = pubKey

		// Create emission with proper authorization
		addresses := []string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
		amounts := []int64{expectedEmissionAmount}
		tx := createTestEmissionTx(t, addresses, amounts, cointype.CoinType(1), params)

		auth := &chaincfg.SKAEmissionAuth{
			EmissionKey: pubKey,
			CoinType:    cointype.CoinType(1),
			Nonce:       1,
			Amount:      expectedEmissionAmount,
			Height:      int64(config.EmissionHeight),
		}
		signEmissionTx(t, tx, auth, privKey, params)

		// Modify to use VAR output (should fail validation)
		tx.TxOut[0].CoinType = cointype.CoinTypeVAR

		err = ValidateAuthorizedSKAEmissionTransaction(tx, int64(config.EmissionHeight), chain, params)
		if err == nil {
			t.Error("Emission with VAR output should fail")
		}
	})
}

// TestSKAEmissionWindowCalculations tests window calculations.
func TestSKAEmissionWindowCalculations(t *testing.T) {
	tests := []struct {
		name           string
		emissionHeight int64
		emissionWindow int64
		currentHeight  int64
		expectedActive bool
	}{
		{
			name:           "At window start",
			emissionHeight: 100,
			emissionWindow: 50,
			currentHeight:  100,
			expectedActive: true,
		},
		{
			name:           "Within window",
			emissionHeight: 100,
			emissionWindow: 50,
			currentHeight:  125,
			expectedActive: true,
		},
		{
			name:           "At window end",
			emissionHeight: 100,
			emissionWindow: 50,
			currentHeight:  150,
			expectedActive: true,
		},
		{
			name:           "Before window",
			emissionHeight: 100,
			emissionWindow: 50,
			currentHeight:  99,
			expectedActive: false,
		},
		{
			name:           "After window",
			emissionHeight: 100,
			emissionWindow: 50,
			currentHeight:  151,
			expectedActive: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := &chaincfg.Params{
				SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
					1: {
						CoinType:       1,
						EmissionHeight: int32(test.emissionHeight),
						EmissionWindow: int32(test.emissionWindow),
					},
				},
			}

			isActive := isSKAEmissionWindow(test.currentHeight, 1, params)
			if isActive != test.expectedActive {
				t.Errorf("Expected %t, got %t", test.expectedActive, isActive)
			}
		})
	}
}

// TestSKAEmissionConcurrency tests basic concurrency scenarios.
func TestSKAEmissionConcurrency(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Get SKA coin type 1 for consistent testing
	config := params.SKACoins[1]
	if config == nil {
		t.Fatal("SKA coin type 1 not found in simnet params")
	}

	t.Run("MultipleEmissions", func(t *testing.T) {
		// Calculate expected total emission amount from config
		var expectedEmissionAmount int64
		for _, amount := range config.EmissionAmounts {
			expectedEmissionAmount += amount
		}

		// Test that creating emission transactions with partial amounts fails
		// This tests the amount validation in CreateAuthorizedSKAEmissionTransaction
		emissionKey := params.GetSKAEmissionKey(cointype.CoinType(1))
		if emissionKey == nil {
			t.Fatal("No emission key configured for SKA-1")
		}

		auth1 := &chaincfg.SKAEmissionAuth{
			EmissionKey: emissionKey,
			Signature:   make([]byte, 64), // Dummy signature
			Nonce:       1,
			CoinType:    cointype.CoinType(1),
			Amount:      expectedEmissionAmount, // Auth for full amount, but tx has partial
			Height:      10,
		}
		_, err1 := CreateAuthorizedSKAEmissionTransaction(
			auth1,
			[]string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			[]int64{expectedEmissionAmount / 2},
			params,
		)

		auth2 := &chaincfg.SKAEmissionAuth{
			EmissionKey: emissionKey,
			Signature:   make([]byte, 64), // Dummy signature
			Nonce:       2,
			CoinType:    cointype.CoinType(1),
			Amount:      expectedEmissionAmount, // Auth for full amount, but tx has partial
			Height:      10,
		}
		_, err2 := CreateAuthorizedSKAEmissionTransaction(
			auth2,
			[]string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			[]int64{expectedEmissionAmount / 2},
			params,
		)

		// Both should fail because they don't have the complete emission amount
		if err1 == nil {
			t.Error("Expected partial emission tx1 to fail")
		}
		if err2 == nil {
			t.Error("Expected partial emission tx2 to fail")
		}
	})
}
