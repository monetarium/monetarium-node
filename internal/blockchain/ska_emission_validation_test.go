// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

// TestSKACoinTypeValidation tests validation of SKA coin types in transactions.
func TestSKACoinTypeValidation(t *testing.T) {
	// Create a test chain parameters with custom SKA configurations
	params := chaincfg.MainNetParams()

	// Mock transaction for testing (simplified structure)
	createMockTx := func(outputs []wire.TxOut) *wire.MsgTx {
		tx := &wire.MsgTx{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0,
					},
					SignatureScript: []byte{},
					Sequence:        wire.MaxTxInSequenceNum,
				},
			},
			TxOut:    make([]*wire.TxOut, len(outputs)),
			LockTime: 0,
			Expiry:   0,
		}
		for i, output := range outputs {
			tx.TxOut[i] = &output
		}
		return tx
	}

	tests := []struct {
		name        string
		outputs     []wire.TxOut
		shouldError bool
		errorSubstr string
	}{
		{
			name: "Valid VAR output",
			outputs: []wire.TxOut{
				{
					Value:    100000000, // 1 VAR
					CoinType: cointype.CoinTypeVAR,
					PkScript: []byte{0x76, 0xa9, 0x14}, // Mock script
				},
			},
			shouldError: false,
		},
		{
			name: "Valid active SKA-1 output",
			outputs: []wire.TxOut{
				{
					Value:    100000000,                // 1 SKA-1
					CoinType: cointype.CoinType(1),     // SKA-1 = 1
					PkScript: []byte{0x76, 0xa9, 0x14}, // Mock script
				},
			},
			shouldError: false,
		},
		{
			name: "Invalid inactive SKA-2 output",
			outputs: []wire.TxOut{
				{
					Value:    100000000,                // 1 SKA-2
					CoinType: cointype.CoinType(2),     // SKA-2 is configured but inactive
					PkScript: []byte{0x76, 0xa9, 0x14}, // Mock script
				},
			},
			shouldError: true,
			errorSubstr: "inactive SKA coin type",
		},
		{
			name: "Invalid unconfigured SKA-99 output",
			outputs: []wire.TxOut{
				{
					Value:    100000000,                // 1 SKA-99
					CoinType: cointype.CoinType(99),    // SKA-99 is not configured
					PkScript: []byte{0x76, 0xa9, 0x14}, // Mock script
				},
			},
			shouldError: true,
			errorSubstr: "inactive SKA coin type",
		},
		{
			name: "Invalid coin type > 255",
			outputs: []wire.TxOut{
				{
					Value:    100000000,
					CoinType: cointype.CoinType(255), // This should be valid but inactive
					PkScript: []byte{0x76, 0xa9, 0x14},
				},
			},
			shouldError: true,
			errorSubstr: "inactive SKA coin type",
		},
		{
			name: "Mixed valid outputs",
			outputs: []wire.TxOut{
				{
					Value:    100000000, // 1 VAR
					CoinType: cointype.CoinTypeVAR,
					PkScript: []byte{0x76, 0xa9, 0x14},
				},
				{
					Value:    50000000, // 0.5 SKA-1
					CoinType: cointype.CoinType(1),
					PkScript: []byte{0x76, 0xa9, 0x14},
				},
			},
			shouldError: false,
		},
		{
			name: "Mixed valid and invalid outputs",
			outputs: []wire.TxOut{
				{
					Value:    100000000, // 1 VAR
					CoinType: cointype.CoinTypeVAR,
					PkScript: []byte{0x76, 0xa9, 0x14},
				},
				{
					Value:    50000000, // 0.5 SKA-2 (inactive)
					CoinType: cointype.CoinType(2),
					PkScript: []byte{0x76, 0xa9, 0x14},
				},
			},
			shouldError: true,
			errorSubstr: "inactive SKA coin type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create mock transaction
			msgTx := createMockTx(test.outputs)

			// Test the coin type validation logic directly
			// (This simulates the validation that happens in CheckTransactionInputs)
			var hasError bool
			var errorMsg string

			for _, txOut := range msgTx.TxOut {
				coinType := txOut.CoinType
				switch {
				case coinType == cointype.CoinTypeVAR:
					// VAR is always valid
					continue
				case coinType >= cointype.CoinType(1) && coinType <= cointype.CoinTypeMax:
					// Check if this SKA coin type is active
					if !params.IsSKACoinTypeActive(coinType) {
						hasError = true
						errorMsg = "inactive SKA coin type"
						break
					}
				default:
					// Invalid coin type
					hasError = true
					errorMsg = "invalid coin type"
				}
			}

			if test.shouldError {
				if !hasError {
					t.Errorf("Expected error but validation passed")
				} else if test.errorSubstr != "" && !stringContains(errorMsg, test.errorSubstr) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorSubstr, errorMsg)
				}
			} else {
				if hasError {
					t.Errorf("Expected validation to pass but got error: %s", errorMsg)
				}
			}
		})
	}
}

// TestSKACoinTypeStringRepresentation tests the string representation of coin types.
func TestSKACoinTypeStringRepresentation(t *testing.T) {
	tests := []struct {
		coinType cointype.CoinType
		expected string
	}{
		{cointype.CoinTypeVAR, "VAR"},
		{cointype.CoinType(1), "SKA-1"},
		{cointype.CoinType(2), "SKA-2"},
		{cointype.CoinType(25), "SKA-25"},
		{cointype.CoinType(255), "SKA-255"},
	}

	for _, test := range tests {
		result := test.coinType.String()
		if result != test.expected {
			t.Errorf("CoinType(%d).String() = %s, expected %s",
				test.coinType, result, test.expected)
		}
	}
}

// TestSKACoinTypeProperties tests that all SKA variants have consistent properties.
func TestSKACoinTypeProperties(t *testing.T) {
	testCoinTypes := []cointype.CoinType{
		cointype.CoinType(1), // 1
		cointype.CoinType(2),
		cointype.CoinType(25),
		cointype.CoinType(255),
	}

	for _, coinType := range testCoinTypes {
		// Test AtomsPerCoin
		atomsPerCoin := coinType.AtomsPerCoin()
		if atomsPerCoin != int64(cointype.AtomsPerSKA) {
			t.Errorf("CoinType(%d).AtomsPerCoin() = %d, expected %d",
				coinType, atomsPerCoin, int64(cointype.AtomsPerSKA))
		}

		// Test MaxAtoms
		maxAtoms := coinType.MaxAtoms()
		if maxAtoms != int64(cointype.MaxSKAAtoms) {
			t.Errorf("CoinType(%d).MaxAtoms() = %d, expected %d",
				coinType, maxAtoms, int64(cointype.MaxSKAAtoms))
		}

		// Test MaxAmount
		maxAmount := coinType.MaxAmount()
		if maxAmount != cointype.MaxSKAAmount {
			t.Errorf("CoinType(%d).MaxAmount() = %d, expected %d",
				coinType, maxAmount, cointype.MaxSKAAmount)
		}

		// Test IsValid
		if !coinType.IsValid() {
			t.Errorf("CoinType(%d).IsValid() = false, expected true", coinType)
		}
	}
}

// TestVARCoinTypeProperties tests VAR coin type properties.
func TestVARCoinTypeProperties(t *testing.T) {
	coinType := cointype.CoinTypeVAR

	// Test AtomsPerCoin
	atomsPerCoin := coinType.AtomsPerCoin()
	if atomsPerCoin != int64(cointype.AtomsPerVAR) {
		t.Errorf("VAR AtomsPerCoin() = %d, expected %d",
			atomsPerCoin, int64(cointype.AtomsPerVAR))
	}

	// Test MaxAtoms
	maxAtoms := coinType.MaxAtoms()
	if maxAtoms != int64(cointype.MaxVARAtoms) {
		t.Errorf("VAR MaxAtoms() = %d, expected %d",
			maxAtoms, int64(cointype.MaxVARAtoms))
	}

	// Test MaxAmount
	maxAmount := coinType.MaxAmount()
	if maxAmount != cointype.MaxVARAmount {
		t.Errorf("VAR MaxAmount() = %d, expected %d",
			maxAmount, cointype.MaxVARAmount)
	}

	// Test IsValid
	if !coinType.IsValid() {
		t.Error("VAR coin type should be valid")
	}

	// Test String
	if coinType.String() != "VAR" {
		t.Errorf("VAR String() = %s, expected VAR", coinType.String())
	}
}

// TestCoinTypeValidationRange tests the full range of coin type validation.
func TestCoinTypeValidationRange(t *testing.T) {
	tests := []struct {
		coinType uint8
		isValid  bool
		name     string
	}{
		{0, true, "VAR (0)"},
		{1, true, "SKA-1 (1)"},
		{2, true, "SKA-2 (2)"},
		{99, true, "SKA-99 (99)"},
		{255, true, "SKA-255 (255)"},
	}

	for _, test := range tests {
		coinType := cointype.CoinType(test.coinType)
		isValid := coinType.IsValid()
		if isValid != test.isValid {
			t.Errorf("%s: IsValid() = %t, expected %t",
				test.name, isValid, test.isValid)
		}
	}
}

// Simple string contains check since we can't import strings package
func stringContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
