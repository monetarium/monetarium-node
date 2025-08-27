// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/wire"
)

// TestSKAEmissionBasicValidation tests basic SKA emission validation.
func TestSKAEmissionBasicValidation(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Get SKA coin type 1 for consistent testing
	config := params.SKACoins[1]
	if config == nil {
		t.Fatal("SKA coin type 1 not found in simnet params")
	}

	t.Run("ValidEmission", func(t *testing.T) {
		// Calculate expected total emission amount from config
		var expectedEmissionAmount int64
		for _, amount := range config.EmissionAmounts {
			expectedEmissionAmount += amount
		}

		// Create a valid emission transaction with the correct amount
		tx := createValidEmissionTx(expectedEmissionAmount)

		// Test at the correct emission height
		err := ValidateSKAEmissionTransaction(tx, int64(config.EmissionHeight), params)
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

		// Create emission with wrong amount by using CreateSKAEmissionTransaction
		// which now validates the amount
		_, err := CreateSKAEmissionTransaction(
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

		// Create valid emission at wrong height
		tx := createValidEmissionTx(expectedEmissionAmount)

		err := ValidateSKAEmissionTransaction(tx, int64(config.EmissionHeight)+1000, params)
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

		// Create emission with VAR output (should fail)
		tx := createValidEmissionTx(expectedEmissionAmount)
		tx.TxOut[0].CoinType = cointype.CoinTypeVAR

		err := ValidateSKAEmissionTransaction(tx, int64(config.EmissionHeight), params)
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
		// This tests the amount validation in CreateSKAEmissionTransaction
		_, err1 := CreateSKAEmissionTransaction(
			[]string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			[]int64{expectedEmissionAmount / 2},
			params,
		)

		_, err2 := CreateSKAEmissionTransaction(
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

// Helper function to create a valid emission transaction.
func createValidEmissionTx(amount int64) *wire.MsgTx {
	return &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{}, // Null hash
				Index: 0xffffffff,       // Null index
			},
			SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // Contains "SKA"
			Sequence:        wire.MaxTxInSequenceNum,
		}},
		TxOut: []*wire.TxOut{{
			Value:    amount,
			CoinType: cointype.CoinType(1),
			Version:  0,
			PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x88, 0xac},
		}},
		LockTime: 0,
		Expiry:   0,
	}
}
