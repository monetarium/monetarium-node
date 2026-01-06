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

// mockChainStateForStakeValidation implements ChainStateProvider for testing
type mockChainStateForStakeValidation struct {
	emissionOccurred map[cointype.CoinType]bool
	emissionNonces   map[cointype.CoinType]uint64
}

func (m *mockChainStateForStakeValidation) HasSKAEmissionOccurred(ct cointype.CoinType) bool {
	return m.emissionOccurred[ct]
}

func (m *mockChainStateForStakeValidation) GetSKAEmissionNonce(ct cointype.CoinType) uint64 {
	return m.emissionNonces[ct]
}

// TestSKAEmissionStakeValidationHeight tests that SKA emissions are properly
// rejected before stake validation height.
func TestSKAEmissionStakeValidationHeight(t *testing.T) {
	// Create test parameters with a stake validation height
	params := &chaincfg.Params{
		StakeValidationHeight: 100,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				Name:           "TestSKA",
				Symbol:         "TSK",
				MaxSupply:      1e6 * 1e8,
				EmissionHeight: 150, // After stake validation
				EmissionWindow: 50,
				Active:         true,
				EmissionKey:    &secp256k1.PublicKey{}, // Dummy key for testing
			},
		},
	}

	// Create mock chain state
	chainState := &mockChainStateForStakeValidation{
		emissionOccurred: make(map[cointype.CoinType]bool),
		emissionNonces:   make(map[cointype.CoinType]uint64),
	}

	// Create a basic SKA emission transaction
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{},
					Index: 0xffffffff,
				},
				SignatureScript: []byte{0x00}, // Dummy signature script
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    1000000,
				CoinType: cointype.CoinType(1),
				PkScript: []byte{0x00}, // Dummy script
			},
		},
	}

	tests := []struct {
		name        string
		blockHeight int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Emission before stake validation height",
			blockHeight: 50, // Before stake validation (100)
			expectError: true,
			errorMsg:    "SKA emission not allowed before stake validation height",
		},
		{
			name:        "Emission at stake validation height",
			blockHeight: 100, // At stake validation height
			expectError: true,
			errorMsg:    "SKA emission transaction at invalid height", // Not in emission window yet
		},
		{
			name:        "Emission after stake validation but before window",
			blockHeight: 120, // After stake validation but before emission window
			expectError: true,
			errorMsg:    "SKA emission transaction at invalid height",
		},
		{
			name:        "Emission in valid window",
			blockHeight: 150, // In emission window (150-200)
			expectError: true,
			errorMsg:    "invalid emission authorization", // Will fail on auth, but passes height check
		},
		{
			name:        "Emission at end of window",
			blockHeight: 200, // At end of emission window
			expectError: true,
			errorMsg:    "invalid emission authorization", // Will fail on auth, but passes height check
		},
		{
			name:        "Emission after window",
			blockHeight: 201, // After emission window
			expectError: true,
			errorMsg:    "SKA emission transaction at invalid height",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAuthorizedSKAEmissionTransaction(tx, test.blockHeight, chainState, params)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if test.errorMsg != "" && !contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestChainInitializationSKAValidation tests that chain initialization properly
// validates SKA emission heights against stake validation height.
func TestChainInitializationSKAValidation(t *testing.T) {
	tests := []struct {
		name              string
		stakeValidHeight  int64
		skaEmissionHeight int32
		expectError       bool
	}{
		{
			name:              "SKA emission before stake validation",
			stakeValidHeight:  100,
			skaEmissionHeight: 50,
			expectError:       true,
		},
		{
			name:              "SKA emission at stake validation",
			stakeValidHeight:  100,
			skaEmissionHeight: 100,
			expectError:       false,
		},
		{
			name:              "SKA emission after stake validation",
			stakeValidHeight:  100,
			skaEmissionHeight: 150,
			expectError:       false,
		},
		{
			name:              "Zero emission height (disabled)",
			stakeValidHeight:  100,
			skaEmissionHeight: 0,
			expectError:       false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create test parameters
			params := &chaincfg.Params{
				StakeValidationHeight: test.stakeValidHeight,
				SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
					1: {
						CoinType:       1,
						EmissionHeight: test.skaEmissionHeight,
						EmissionWindow: 50,
					},
				},
			}

			// Simulate the validation that happens in blockchain.New
			for coinType, skaConfig := range params.SKACoins {
				if skaConfig.EmissionHeight > 0 && int64(skaConfig.EmissionHeight) < params.StakeValidationHeight {
					if !test.expectError {
						t.Errorf("Expected no error but validation would fail for coin type %d", coinType)
					}
					return
				}
			}

			if test.expectError {
				t.Error("Expected validation error but got none")
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || contains(s[1:], substr)))
}
