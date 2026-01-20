// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"math/big"
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
	"github.com/monetarium/monetarium-node/wire"
)

// createMockTransaction creates a mock transaction with the specified coin type outputs.
// VAR outputs use Value (int64), SKA outputs use SKAValue (big.Int).
func createMockTransaction(coinTypes []cointype.CoinType) *dcrutil.Tx {
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{},
				SignatureScript:  []byte{},
				Sequence:         wire.MaxTxInSequenceNum,
			},
		},
		TxOut: make([]*wire.TxOut, len(coinTypes)),
	}

	for i, coinType := range coinTypes {
		txOut := &wire.TxOut{
			CoinType: coinType,
			PkScript: []byte{0x51}, // OP_TRUE
		}
		if coinType.IsSKA() {
			// SKA outputs use SKAValue, Value must be 0
			txOut.Value = 0
			txOut.SKAValue = big.NewInt(1000000) // 1 coin in atoms
		} else {
			// VAR outputs use Value
			txOut.Value = 1000000 // 1 coin in atoms
		}
		tx.TxOut[i] = txOut
	}

	return dcrutil.NewTx(tx)
}

// createMockTransactionWithValues creates a transaction with specific values per output.
// VAR outputs use Value (int64), SKA outputs use SKAValue (big.Int).
func createMockTransactionWithValues(outputs []struct {
	coinType cointype.CoinType
	value    int64
}) *dcrutil.Tx {
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{},
				SignatureScript:  []byte{},
				Sequence:         wire.MaxTxInSequenceNum,
			},
		},
		TxOut: make([]*wire.TxOut, len(outputs)),
	}

	for i, out := range outputs {
		txOut := &wire.TxOut{
			CoinType: out.coinType,
			PkScript: []byte{0x51}, // OP_TRUE
		}
		if out.coinType.IsSKA() {
			// SKA outputs use SKAValue, Value must be 0
			txOut.Value = 0
			txOut.SKAValue = big.NewInt(out.value)
		} else {
			// VAR outputs use Value
			txOut.Value = out.value
		}
		tx.TxOut[i] = txOut
	}

	return dcrutil.NewTx(tx)
}

// TestGetTransactionCoinType verifies transaction coin type determination.
// Note: In practice, transactions cannot mix coin types (consensus rule).
// This function returns the coin type of the first output with a positive value.
func TestGetTransactionCoinType(t *testing.T) {
	testCases := []struct {
		name         string
		coinTypes    []cointype.CoinType
		expectedType cointype.CoinType
	}{
		{
			name:         "VAR only transaction",
			coinTypes:    []cointype.CoinType{cointype.CoinTypeVAR, cointype.CoinTypeVAR},
			expectedType: cointype.CoinTypeVAR,
		},
		{
			name:         "SKA-1 only transaction",
			coinTypes:    []cointype.CoinType{cointype.CoinType(1), cointype.CoinType(1)},
			expectedType: cointype.CoinType(1),
		},
		{
			name:         "SKA-2 only transaction",
			coinTypes:    []cointype.CoinType{cointype.CoinType(2), cointype.CoinType(2)},
			expectedType: cointype.CoinType(2),
		},
		{
			name:         "Single VAR output transaction",
			coinTypes:    []cointype.CoinType{cointype.CoinTypeVAR},
			expectedType: cointype.CoinTypeVAR,
		},
		{
			name:         "Single SKA output transaction",
			coinTypes:    []cointype.CoinType{cointype.CoinType(2)},
			expectedType: cointype.CoinType(2),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := createMockTransaction(tc.coinTypes)
			coinType := GetTransactionCoinType(tx)

			if coinType != tc.expectedType {
				t.Errorf("Expected coin type %d, got %d", tc.expectedType, coinType)
			}
		})
	}
}

// TestGetTransactionCoinTypeWithValues tests coin type determination with specific values.
// Note: Transactions cannot mix coin types, but this tests edge cases like zero-value outputs.
func TestGetTransactionCoinTypeWithValues(t *testing.T) {
	testCases := []struct {
		name    string
		outputs []struct {
			coinType cointype.CoinType
			value    int64
		}
		expectedType cointype.CoinType
	}{
		{
			name: "VAR transaction with multiple outputs",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinTypeVAR, 100000},
				{cointype.CoinTypeVAR, 200000},
				{cointype.CoinTypeVAR, 300000},
			},
			expectedType: cointype.CoinTypeVAR,
		},
		{
			name: "SKA transaction with multiple outputs",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinType(1), 500000},
				{cointype.CoinType(1), 600000},
			},
			expectedType: cointype.CoinType(1),
		},
		{
			name: "Zero value output followed by positive value - same type",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinTypeVAR, 0},   // zero value
				{cointype.CoinTypeVAR, 100}, // positive value
			},
			expectedType: cointype.CoinTypeVAR, // returns first positive value output
		},
		{
			name: "SKA zero value output followed by positive value",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinType(2), 0},   // zero value SKA
				{cointype.CoinType(2), 100}, // positive value SKA
			},
			expectedType: cointype.CoinType(2),
		},
		{
			name: "All zero value outputs - uses first output type",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinType(1), 0},
				{cointype.CoinType(1), 0},
			},
			expectedType: cointype.CoinType(1), // fallback to first output
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := createMockTransactionWithValues(tc.outputs)
			coinType := GetTransactionCoinType(tx)

			if coinType != tc.expectedType {
				t.Errorf("Expected coin type %d, got %d", tc.expectedType, coinType)
			}
		})
	}
}

// TestTransactionSizeTracker verifies transaction size tracking functionality.
func TestTransactionSizeTracker(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block
	tracker := NewTransactionSizeTracker(allocator)

	// Create test transactions
	varTx := createMockTransaction([]cointype.CoinType{cointype.CoinTypeVAR, cointype.CoinTypeVAR})
	ska1Tx := createMockTransaction([]cointype.CoinType{cointype.CoinType(1), cointype.CoinType(1)})
	ska2Tx := createMockTransaction([]cointype.CoinType{cointype.CoinType(2)})

	// Add transactions to tracker
	tracker.AddTransaction(varTx)
	tracker.AddTransaction(ska1Tx)
	tracker.AddTransaction(ska2Tx)

	// Verify sizes are tracked correctly
	varSize := tracker.GetSizeForCoinType(cointype.CoinTypeVAR)
	if varSize == 0 {
		t.Error("Expected VAR size to be tracked")
	}

	ska1Size := tracker.GetSizeForCoinType(1)
	if ska1Size == 0 {
		t.Error("Expected SKA-1 size to be tracked")
	}

	ska2Size := tracker.GetSizeForCoinType(2)
	if ska2Size == 0 {
		t.Error("Expected SKA-2 size to be tracked")
	}

	// Verify allocation calculation
	allocation := tracker.GetAllocation()
	if allocation == nil {
		t.Fatal("Expected allocation result")
	}

	if allocation.TotalUsed == 0 {
		t.Error("Expected non-zero total usage")
	}
}

// TestCanAddTransaction verifies transaction addition validation.
func TestCanAddTransaction(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000, params) // Small 1KB block for testing
	tracker := NewTransactionSizeTracker(allocator)

	// Create a transaction that would fill most of the VAR allocation
	varTx := createMockTransaction([]cointype.CoinType{cointype.CoinTypeVAR})

	// First transaction should be addable
	if !tracker.CanAddTransaction(varTx) {
		t.Error("First VAR transaction should be addable")
	}

	// Add the transaction
	tracker.AddTransaction(varTx)

	// Create a very large transaction that would exceed allocation
	largeCoinTypes := make([]cointype.CoinType, 100) // Large transaction
	for i := range largeCoinTypes {
		largeCoinTypes[i] = cointype.CoinTypeVAR
	}
	largeTx := createMockTransaction(largeCoinTypes)

	// Large transaction should not be addable
	if tracker.CanAddTransaction(largeTx) {
		t.Error("Large transaction should not be addable when it would exceed allocation")
	}
}

// TestTrackerReset verifies the reset functionality.
func TestTrackerReset(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params)
	tracker := NewTransactionSizeTracker(allocator)

	// Add a transaction
	varTx := createMockTransaction([]cointype.CoinType{cointype.CoinTypeVAR})
	tracker.AddTransaction(varTx)

	// Verify transaction is tracked
	if tracker.GetSizeForCoinType(cointype.CoinTypeVAR) == 0 {
		t.Error("Expected VAR size to be tracked before reset")
	}

	// Reset tracker
	tracker.Reset()

	// Verify all sizes are cleared
	if tracker.GetSizeForCoinType(cointype.CoinTypeVAR) != 0 {
		t.Error("Expected VAR size to be 0 after reset")
	}
}
