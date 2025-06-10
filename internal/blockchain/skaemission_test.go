// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/wire"
)

// TestSKAEmissionBlockDetection tests the emission block detection logic.
func TestSKAEmissionBlockDetection(t *testing.T) {
	// Use SimNet parameters for testing
	params := chaincfg.SimNetParams()

	tests := []struct {
		name        string
		blockHeight int64
		expected    bool
	}{
		{
			name:        "Before emission height",
			blockHeight: 5,
			expected:    false,
		},
		{
			name:        "At emission height",
			blockHeight: 10, // SimNet emission height
			expected:    true,
		},
		{
			name:        "After emission height",
			blockHeight: 15,
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isSKAEmissionBlock(test.blockHeight, params)
			if result != test.expected {
				t.Errorf("isSKAEmissionBlock(%d): expected %t, got %t",
					test.blockHeight, test.expected, result)
			}
		})
	}
}

// TestSKAActivation tests the SKA activation logic.
func TestSKAActivation(t *testing.T) {
	// Use SimNet parameters for testing
	params := chaincfg.SimNetParams()

	tests := []struct {
		name        string
		blockHeight int64
		expected    bool
	}{
		{
			name:        "Before activation height",
			blockHeight: 5,
			expected:    false,
		},
		{
			name:        "At activation height",
			blockHeight: 10, // SimNet activation height
			expected:    true,
		},
		{
			name:        "After activation height",
			blockHeight: 15,
			expected:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isSKAActive(test.blockHeight, params)
			if result != test.expected {
				t.Errorf("isSKAActive(%d): expected %t, got %t",
					test.blockHeight, test.expected, result)
			}
		})
	}
}

// TestCreateSKAEmissionTransactionValidation tests the validation logic 
// for SKA emission transaction creation without requiring valid addresses.
func TestCreateSKAEmissionTransactionValidation(t *testing.T) {
	params := chaincfg.SimNetParams()

	tests := []struct {
		name        string
		addresses   []string
		amounts     []int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Mismatched addresses and amounts",
			addresses:   []string{"addr1", "addr2"},
			amounts:     []int64{50000000000000}, // Only one amount
			expectError: true,
			errorMsg:    "length mismatch",
		},
		{
			name:        "No addresses",
			addresses:   []string{},
			amounts:     []int64{},
			expectError: true,
			errorMsg:    "no emission addresses",
		},
		{
			name:        "Invalid amount (zero)",
			addresses:   []string{"addr1", "addr2"},
			amounts:     []int64{0, 100000000000000},
			expectError: true,
			errorMsg:    "invalid emission amount",
		},
		{
			name:        "Wrong total amount",
			addresses:   []string{"addr1", "addr2"},
			amounts:     []int64{25000000000000, 25000000000000}, // 500,000 total (wrong)
			expectError: true,
			errorMsg:    "does not match chain parameter",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CreateSKAEmissionTransaction(test.addresses, test.amounts, params)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got none", test.errorMsg)
					return
				}
				if len(test.errorMsg) > 0 && !contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestIsSKAEmissionTransaction tests the detection of SKA emission transactions.
func TestIsSKAEmissionTransaction(t *testing.T) {
	tests := []struct {
		name     string
		tx       *wire.MsgTx
		expected bool
	}{
		{
			name: "Valid SKA emission transaction",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{}, // Null hash
						Index: 0xffffffff,      // Null index
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // Contains "SKA"
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: wire.CoinTypeSKA,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: true,
		},
		{
			name: "Multiple inputs",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{},
							Index: 0xffffffff,
						},
						SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
					},
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{0x01}, // Non-null
							Index: 0,
						},
						SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
					},
				},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: wire.CoinTypeSKA,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // Multiple inputs not allowed
		},
		{
			name: "Non-null input",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{0x01}, // Non-null
						Index: 0,
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: wire.CoinTypeSKA,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // Input not null
		},
		{
			name: "Missing SKA marker",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{0x01, 0x02, 0x03}, // No "SKA"
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: wire.CoinTypeSKA,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // Missing SKA marker
		},
		{
			name: "VAR output instead of SKA",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: wire.CoinTypeVAR, // Wrong coin type
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // VAR output not allowed
		},
		{
			name: "No outputs",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
				}},
				TxOut: []*wire.TxOut{}, // No outputs
			},
			expected: false, // Must have outputs
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsSKAEmissionTransaction(test.tx)
			if result != test.expected {
				t.Errorf("IsSKAEmissionTransaction: expected %t, got %t", test.expected, result)
			}
		})
	}
}

// TestValidateSKAEmissionTransaction tests the validation of SKA emission transactions.
func TestValidateSKAEmissionTransaction(t *testing.T) {
	params := chaincfg.SimNetParams()
	emissionHeight := params.SKAEmissionHeight

	// Create a valid emission transaction
	validTx := &wire.MsgTx{
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // "SKA" marker
		}},
		TxOut: []*wire.TxOut{{
			Value:    params.SKAEmissionAmount, // Full emission amount
			CoinType: wire.CoinTypeSKA,
			Version:  0,
			PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
		}},
		LockTime: 0,
		Expiry:   0,
	}

	tests := []struct {
		name        string
		tx          *wire.MsgTx
		blockHeight int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid emission transaction",
			tx:          validTx,
			blockHeight: emissionHeight,
			expectError: false,
		},
		{
			name:        "Wrong block height",
			tx:          validTx,
			blockHeight: emissionHeight + 1, // Wrong height
			expectError: true,
			errorMsg:    "invalid height",
		},
		{
			name: "Multiple inputs",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					validTx.TxIn[0],
					validTx.TxIn[0], // Duplicate input
				},
				TxOut:    validTx.TxOut,
				LockTime: 0,
				Expiry:   0,
			},
			blockHeight: emissionHeight,
			expectError: true,
			errorMsg:    "exactly 1 input",
		},
		{
			name: "Wrong total amount",
			tx: &wire.MsgTx{
				TxIn: validTx.TxIn,
				TxOut: []*wire.TxOut{{
					Value:    params.SKAEmissionAmount / 2, // Wrong amount
					CoinType: wire.CoinTypeSKA,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
				LockTime: 0,
				Expiry:   0,
			},
			blockHeight: emissionHeight,
			expectError: true,
			errorMsg:    "does not match chain parameter",
		},
		{
			name: "Non-zero locktime",
			tx: &wire.MsgTx{
				TxIn:     validTx.TxIn,
				TxOut:    validTx.TxOut,
				LockTime: 100, // Non-zero locktime
				Expiry:   0,
			},
			blockHeight: emissionHeight,
			expectError: true,
			errorMsg:    "LockTime 0",
		},
		{
			name: "Non-zero expiry",
			tx: &wire.MsgTx{
				TxIn:     validTx.TxIn,
				TxOut:    validTx.TxOut,
				LockTime: 0,
				Expiry:   100, // Non-zero expiry
			},
			blockHeight: emissionHeight,
			expectError: true,
			errorMsg:    "Expiry 0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateSKAEmissionTransaction(test.tx, test.blockHeight, params)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got none", test.errorMsg)
					return
				}
				if !contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (substr == "" || 
		    (len(s) > 0 && (s == substr || 
		                    (len(s) > len(substr) && 
		                     (s[:len(substr)] == substr || 
		                      s[len(s)-len(substr):] == substr || 
		                      indexStr(s, substr) >= 0)))))
}

// indexStr finds the index of substr in s, returns -1 if not found.
func indexStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}