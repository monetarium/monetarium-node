// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
)

// GetTransactionCoinType determines the primary coin type of a transaction
// based on the coin type of outputs. Since transactions cannot mix coin types
// (all outputs must have the same coin type), we return the coin type of the
// first non-null-data output.
func GetTransactionCoinType(tx *dcrutil.Tx) cointype.CoinType {
	msgTx := tx.MsgTx()
	if len(msgTx.TxOut) == 0 {
		return cointype.CoinTypeVAR // Default to VAR for transactions with no outputs
	}

	// Find the first output with a value (skip OP_RETURN/null data outputs)
	// All outputs in a transaction must have the same coin type, so we just
	// need to find one with actual value to determine the type.
	for _, txOut := range msgTx.TxOut {
		// For SKA outputs, Value=0 and SKAValue contains the actual amount
		if txOut.CoinType.IsSKA() {
			if txOut.SKAValue != nil && txOut.SKAValue.Sign() > 0 {
				return txOut.CoinType
			}
		} else {
			// VAR output - check Value field
			if txOut.Value > 0 {
				return txOut.CoinType
			}
		}
	}

	// Fallback: return the coin type of the first output even if value is 0
	// This handles edge cases like OP_RETURN-only transactions
	return msgTx.TxOut[0].CoinType
}

// TransactionSizeTracker tracks transaction sizes by coin type for block space allocation.
type TransactionSizeTracker struct {
	sizesByCoinType map[cointype.CoinType]uint32
	allocator       *BlockSpaceAllocator
}

// NewTransactionSizeTracker creates a new transaction size tracker.
func NewTransactionSizeTracker(allocator *BlockSpaceAllocator) *TransactionSizeTracker {
	return &TransactionSizeTracker{
		sizesByCoinType: make(map[cointype.CoinType]uint32),
		allocator:       allocator,
	}
}

// AddTransaction adds a transaction to the size tracking.
func (tst *TransactionSizeTracker) AddTransaction(tx *dcrutil.Tx) {
	coinType := GetTransactionCoinType(tx)
	txSize := uint32(tx.MsgTx().SerializeSize())
	tst.sizesByCoinType[coinType] += txSize
}

// GetAllocation returns the current block space allocation based on tracked transaction sizes.
func (tst *TransactionSizeTracker) GetAllocation() *AllocationResult {
	return tst.allocator.AllocateBlockSpace(tst.sizesByCoinType)
}

// CanAddTransaction checks if a transaction can be added without exceeding coin type allocation.
func (tst *TransactionSizeTracker) CanAddTransaction(tx *dcrutil.Tx) bool {
	coinType := GetTransactionCoinType(tx)
	txSize := uint32(tx.MsgTx().SerializeSize())

	// Create a temporary copy of current sizes to test the addition
	testSizes := make(map[cointype.CoinType]uint32)
	for ct, size := range tst.sizesByCoinType {
		testSizes[ct] = size
	}
	testSizes[coinType] += txSize

	// Get allocation with the test transaction added
	allocation := tst.allocator.AllocateBlockSpace(testSizes)

	// Check if this coin type would exceed its final allocation
	coinAllocation := allocation.GetAllocationForCoinType(coinType)
	if coinAllocation == nil {
		return false
	}

	return testSizes[coinType] <= coinAllocation.FinalAllocation
}

// GetSizeForCoinType returns the current size tracked for a specific coin type.
func (tst *TransactionSizeTracker) GetSizeForCoinType(coinType cointype.CoinType) uint32 {
	return tst.sizesByCoinType[coinType]
}

// Reset clears all tracked transaction sizes.
func (tst *TransactionSizeTracker) Reset() {
	tst.sizesByCoinType = make(map[cointype.CoinType]uint32)
}
