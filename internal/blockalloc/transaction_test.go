// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
	"github.com/monetarium/monetarium-node/wire"
)

// createMockTransaction creates a mock transaction with the specified coin type outputs.
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
		tx.TxOut[i] = &wire.TxOut{
			Value:    1000000, // 1 coin in atoms
			CoinType: coinType,
			PkScript: []byte{0x51}, // OP_TRUE
		}
	}

	return dcrutil.NewTx(tx)
}

// createMockTransactionWithValues creates a transaction with specific values per output.
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
		tx.TxOut[i] = &wire.TxOut{
			Value:    out.value,
			CoinType: out.coinType,
			PkScript: []byte{0x51}, // OP_TRUE
		}
	}

	return dcrutil.NewTx(tx)
}

// TestGetTransactionCoinType verifies transaction coin type determination.
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
			name:         "Mixed transaction - VAR majority",
			coinTypes:    []cointype.CoinType{cointype.CoinTypeVAR, cointype.CoinTypeVAR, cointype.CoinType(1)},
			expectedType: cointype.CoinTypeVAR,
		},
		{
			name:         "Mixed transaction - SKA majority",
			coinTypes:    []cointype.CoinType{cointype.CoinTypeVAR, cointype.CoinType(1), cointype.CoinType(1)},
			expectedType: cointype.CoinType(1),
		},
		{
			name:         "Single output transaction",
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

// TestGetTransactionCoinTypeValueWeighted tests value-weighted coin type determination.
func TestGetTransactionCoinTypeValueWeighted(t *testing.T) {
	testCases := []struct {
		name    string
		outputs []struct {
			coinType cointype.CoinType
			value    int64
		}
		expectedType cointype.CoinType
	}{
		{
			name: "Many dust outputs vs one large output",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinTypeVAR, 1},       // dust
				{cointype.CoinTypeVAR, 1},       // dust
				{cointype.CoinTypeVAR, 1},       // dust
				{cointype.CoinType(1), 1000000}, // 1 coin
			},
			expectedType: cointype.CoinType(1), // SKA-1 wins by value
		},
		{
			name: "Equal count but different values",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinTypeVAR, 100000}, // 0.1 coin
				{cointype.CoinTypeVAR, 200000}, // 0.2 coin
				{cointype.CoinType(2), 500000}, // 0.5 coin
				{cointype.CoinType(2), 600000}, // 0.6 coin
			},
			expectedType: cointype.CoinType(2), // SKA-2 wins: 1.1 vs 0.3
		},
		{
			name: "Zero value outputs should still count",
			outputs: []struct {
				coinType cointype.CoinType
				value    int64
			}{
				{cointype.CoinTypeVAR, 0},
				{cointype.CoinType(1), 1},
			},
			expectedType: cointype.CoinType(1),
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
