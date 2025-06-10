// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package standalone

import (
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/wire"
)

// TestCheckTransactionSanityDualCoin tests transaction sanity checking with
// dual-coin support (VAR and SKA).
func TestCheckTransactionSanityDualCoin(t *testing.T) {
	maxTxSize := uint64(1000000) // 1MB max transaction size

	tests := []struct {
		name        string
		tx          *wire.MsgTx
		expectError bool
		errorType   string
	}{
		{
			name: "Valid VAR transaction",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    100000000, // 1 VAR
						CoinType: wire.CoinType(CoinTypeVAR),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid SKA transaction",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    50000000, // 0.5 SKA
						CoinType: wire.CoinType(CoinTypeSKA),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid mixed VAR/SKA transaction",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    100000000, // 1 VAR
						CoinType: wire.CoinType(CoinTypeVAR),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
					{
						Value:    200000000, // 2 SKA
						CoinType: wire.CoinType(CoinTypeSKA),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid coin type",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    100000000,
						CoinType: wire.CoinType(99), // Invalid coin type
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
				},
			},
			expectError: true,
			errorType:   "ErrBadTxOutValue",
		},
		{
			name: "VAR amount exceeds maximum",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    maxAtoms + 1, // Exceeds VAR maximum
						CoinType: wire.CoinType(CoinTypeVAR),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
				},
			},
			expectError: true,
			errorType:   "ErrBadTxOutValue",
		},
		{
			name: "SKA amount exceeds maximum",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    maxSKAAtoms + 1, // Exceeds SKA maximum
						CoinType: wire.CoinType(CoinTypeSKA),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
				},
			},
			expectError: true,
			errorType:   "ErrBadTxOutValue",
		},
		{
			name: "Negative amount",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    -1, // Negative amount
						CoinType: wire.CoinType(CoinTypeVAR),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
				},
			},
			expectError: true,
			errorType:   "ErrBadTxOutValue",
		},
		{
			name: "Total VAR outputs exceed maximum",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    maxAtoms / 2 + 1,
						CoinType: wire.CoinType(CoinTypeVAR),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
					{
						Value:    maxAtoms / 2 + 1,
						CoinType: wire.CoinType(CoinTypeVAR),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
				},
			},
			expectError: true,
			errorType:   "ErrBadTxOutValue",
		},
		{
			name: "Total SKA outputs exceed maximum",
			tx: &wire.MsgTx{
				Version: 1,
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{1, 2, 3},
							Index: 0,
							Tree:  0,
						},
						Sequence:        0xffffffff,
						SignatureScript: []byte{0x01},
					},
				},
				TxOut: []*wire.TxOut{
					{
						Value:    maxSKAAtoms / 2 + 1,
						CoinType: wire.CoinType(CoinTypeSKA),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
					{
						Value:    maxSKAAtoms / 2 + 1,
						CoinType: wire.CoinType(CoinTypeSKA),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
				},
			},
			expectError: true,
			errorType:   "ErrBadTxOutValue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CheckTransactionSanity(test.tx, maxTxSize)
			
			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				// Check if error is of expected type (basic string matching)
				if test.errorType != "" && !containsString(err.Error(), "transaction output") {
					t.Errorf("Expected error type %s, got: %v", test.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestCoinTypeValidation tests the coin type validation functions.
func TestCoinTypeValidation(t *testing.T) {
	tests := []struct {
		name     string
		coinType CoinType
		isValid  bool
		maxAtoms int64
	}{
		{"VAR coin type", CoinTypeVAR, true, maxAtoms},
		{"SKA coin type", CoinTypeSKA, true, maxSKAAtoms},
		{"Invalid coin type 2", CoinType(2), false, 0},
		{"Invalid coin type 99", CoinType(99), false, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test isValidCoinType
			isValid := isValidCoinType(test.coinType)
			if isValid != test.isValid {
				t.Errorf("isValidCoinType(%d): expected %t, got %t", 
					test.coinType, test.isValid, isValid)
			}

			// Test getMaxAtomsForCoinType
			maxAtoms := getMaxAtomsForCoinType(test.coinType)
			if maxAtoms != test.maxAtoms {
				t.Errorf("getMaxAtomsForCoinType(%d): expected %d, got %d",
					test.coinType, test.maxAtoms, maxAtoms)
			}
		})
	}
}

// containsString checks if s contains substr (helper function).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) > len(substr) && containsString(s[1:], substr)
}