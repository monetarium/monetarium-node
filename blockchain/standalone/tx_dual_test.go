// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package standalone

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
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
						CoinType: cointype.CoinTypeVAR,
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
						CoinType: cointype.CoinType(1),
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
						CoinType: cointype.CoinTypeVAR,
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
					{
						Value:    200000000, // 2 SKA
						CoinType: cointype.CoinType(1),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid SKA coin type 99",
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
						CoinType: cointype.CoinType(99), // Valid SKA coin type
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
				},
			},
			expectError: false, // Now valid coin type range
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
						Value:    cointype.MaxVARAtoms + 1, // Exceeds VAR maximum
						CoinType: cointype.CoinTypeVAR,
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
						Value:    cointype.MaxSKAAtoms + 1, // Exceeds SKA maximum
						CoinType: cointype.CoinType(1),
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
						CoinType: cointype.CoinTypeVAR,
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
						Value:    cointype.MaxVARAtoms/2 + 1,
						CoinType: cointype.CoinTypeVAR,
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
					},
					{
						Value:    cointype.MaxVARAtoms/2 + 1,
						CoinType: cointype.CoinTypeVAR,
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
						Value:    cointype.MaxSKAAtoms/2 + 1,
						CoinType: cointype.CoinType(1),
						Version:  0,
						PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
					},
					{
						Value:    cointype.MaxSKAAtoms/2 + 1,
						CoinType: cointype.CoinType(1),
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
		coinType cointype.CoinType
		isValid  bool
		maxAtoms int64
	}{
		{"VAR coin type", cointype.CoinTypeVAR, true, cointype.MaxVARAtoms},
		{"SKA coin type", 1, true, cointype.MaxSKAAtoms},
		{"SKA coin type 2", cointype.CoinType(2), true, cointype.MaxSKAAtoms},
		{"SKA coin type 99", cointype.CoinType(99), true, cointype.MaxSKAAtoms},
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
