// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/dcrutil/v4"
)

// GetTransactionCoinType determines the primary coin type of a transaction
// based on the total value of outputs for each coin type.
func GetTransactionCoinType(tx *dcrutil.Tx) cointype.CoinType {
	msgTx := tx.MsgTx()
	if len(msgTx.TxOut) == 0 {
		return cointype.CoinTypeVAR // Default to VAR for transactions with no outputs
	}

	// Sum output values by coin type
	coinTypeValues := make(map[cointype.CoinType]uint64)
	for _, txOut := range msgTx.TxOut {
		coinTypeValues[txOut.CoinType] += uint64(txOut.Value)
	}

	// Find the coin type with the highest total value
	var primaryCoinType cointype.CoinType = cointype.CoinTypeVAR
	var maxValue uint64 = 0

	for coinType, value := range coinTypeValues {
		if value > maxValue {
			maxValue = value
			primaryCoinType = coinType
		}
	}

	return primaryCoinType
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
