// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
)

// BlockSpaceAllocator manages the allocation of block space among different coin types
// following the 10% VAR / 90% SKA proportional distribution strategy.
type BlockSpaceAllocator struct {
	// Maximum block size in bytes
	maxBlockSize uint32

	// VAR allocation percentage (0.10 = 10%)
	varAllocation float64

	// SKA allocation percentage (0.90 = 90%)
	skaAllocation float64

	// Chain parameters for accessing active SKA types
	chainParams *chaincfg.Params
}

// NewBlockSpaceAllocator creates a new block space allocator with the standard
// 10% VAR / 90% SKA allocation strategy.
func NewBlockSpaceAllocator(maxBlockSize uint32, chainParams *chaincfg.Params) *BlockSpaceAllocator {
	return &BlockSpaceAllocator{
		maxBlockSize:  maxBlockSize,
		varAllocation: 0.10, // 10% for VAR
		skaAllocation: 0.90, // 90% for SKA
		chainParams:   chainParams,
	}
}

// CoinTypeAllocation represents the space allocation for a specific coin type.
type CoinTypeAllocation struct {
	CoinType        dcrutil.CoinType
	BaseAllocation  uint32 // Guaranteed space allocation
	FinalAllocation uint32 // Final space after overflow distribution
	PendingBytes    uint32 // Bytes of transactions pending for this coin type
	UsedBytes       uint32 // Bytes actually used by this coin type
}

// AllocationResult contains the complete block space allocation for all coin types.
type AllocationResult struct {
	Allocations     map[dcrutil.CoinType]*CoinTypeAllocation
	TotalAllocated  uint32
	TotalUsed       uint32
	OverflowHandled uint32
}

// AllocateBlockSpace calculates the optimal block space allocation given pending
// transaction sizes for each coin type. Returns allocation details for all coin types.
func (bsa *BlockSpaceAllocator) AllocateBlockSpace(pendingTxBytes map[dcrutil.CoinType]uint32) *AllocationResult {
	// Phase 1: Calculate base allocations
	baseAllocations := bsa.calculateBaseAllocations()

	// Phase 2: Fill base allocations and calculate usage
	allocations := make(map[dcrutil.CoinType]*CoinTypeAllocation)
	totalOverflow := uint32(0)

	for coinType, baseSpace := range baseAllocations {
		pending := pendingTxBytes[coinType]
		used := min(pending, baseSpace)

		allocation := &CoinTypeAllocation{
			CoinType:        coinType,
			BaseAllocation:  baseSpace,
			FinalAllocation: baseSpace,
			PendingBytes:    pending,
			UsedBytes:       used,
		}

		allocations[coinType] = allocation

		// Track unused space for overflow redistribution
		if used < baseSpace {
			totalOverflow += baseSpace - used
		}
	}

	// Phase 3: Identify coin types with remaining demand
	activePendingTypes := bsa.identifyActivePendingTypes(pendingTxBytes, allocations)

	// Phase 4: Distribute overflow space proportionally
	if totalOverflow > 0 && len(activePendingTypes) > 0 {
		bsa.distributeOverflow(allocations, totalOverflow, activePendingTypes)
	}

	// Calculate result summary
	result := &AllocationResult{
		Allocations:     allocations,
		TotalAllocated:  bsa.maxBlockSize,
		OverflowHandled: totalOverflow,
	}

	for _, allocation := range allocations {
		result.TotalUsed += allocation.UsedBytes
	}

	return result
}

// calculateBaseAllocations determines the guaranteed space allocation for each coin type.
func (bsa *BlockSpaceAllocator) calculateBaseAllocations() map[dcrutil.CoinType]uint32 {
	allocations := make(map[dcrutil.CoinType]uint32)

	// VAR gets fixed 10% allocation
	varSpace := uint32(float64(bsa.maxBlockSize) * bsa.varAllocation)
	allocations[dcrutil.CoinTypeVAR] = varSpace

	// SKA types share 90% equally among active types
	activeSKATypes := bsa.chainParams.GetActiveSKATypes()
	if len(activeSKATypes) > 0 {
		totalSKASpace := uint32(float64(bsa.maxBlockSize) * bsa.skaAllocation)
		skaSpacePerType := totalSKASpace / uint32(len(activeSKATypes))

		for _, skaType := range activeSKATypes {
			allocations[skaType] = skaSpacePerType
		}
	}

	return allocations
}

// identifyActivePendingTypes finds coin types that have pending transactions
// after their base allocation is filled.
func (bsa *BlockSpaceAllocator) identifyActivePendingTypes(
	pendingTxBytes map[dcrutil.CoinType]uint32,
	allocations map[dcrutil.CoinType]*CoinTypeAllocation) []dcrutil.CoinType {

	var activePending []dcrutil.CoinType

	for coinType, allocation := range allocations {
		remainingDemand := allocation.PendingBytes - allocation.UsedBytes
		if remainingDemand > 0 {
			activePending = append(activePending, coinType)
		}
	}

	return activePending
}

// distributeOverflow distributes unused block space among coin types with pending
// transactions using the same 10%/90% proportional strategy.
func (bsa *BlockSpaceAllocator) distributeOverflow(
	allocations map[dcrutil.CoinType]*CoinTypeAllocation,
	totalOverflow uint32,
	activePendingTypes []dcrutil.CoinType) {

	// Count active pending types by category
	hasVARPending := false
	activeSKACount := 0

	for _, coinType := range activePendingTypes {
		if coinType == dcrutil.CoinTypeVAR {
			hasVARPending = true
		} else {
			activeSKACount++
		}
	}

	// Calculate overflow distribution based on 10%/90% rule
	var varOverflow, skaOverflowPerType uint32

	if hasVARPending && activeSKACount > 0 {
		// Both VAR and SKA have pending - use 10%/90% split
		varOverflow = uint32(float64(totalOverflow) * bsa.varAllocation)
		skaOverflowTotal := uint32(float64(totalOverflow) * bsa.skaAllocation)
		if activeSKACount > 0 {
			skaOverflowPerType = skaOverflowTotal / uint32(activeSKACount)
		}
	} else if hasVARPending {
		// Only VAR has pending - gets 100%
		varOverflow = totalOverflow
		skaOverflowPerType = 0
	} else if activeSKACount > 0 {
		// Only SKA types have pending - split 100% among them
		varOverflow = 0
		skaOverflowPerType = totalOverflow / uint32(activeSKACount)
	}

	// Apply overflow to allocations
	for _, coinType := range activePendingTypes {
		allocation := allocations[coinType]
		remainingDemand := allocation.PendingBytes - allocation.UsedBytes

		var availableOverflow uint32
		if coinType == dcrutil.CoinTypeVAR {
			availableOverflow = varOverflow
		} else {
			availableOverflow = skaOverflowPerType
		}

		// Use as much overflow as needed (up to available overflow)
		additionalUsage := min(remainingDemand, availableOverflow)
		allocation.UsedBytes += additionalUsage
		allocation.FinalAllocation += additionalUsage
	}
}

// GetAllocationForCoinType returns the space allocation for a specific coin type.
func (result *AllocationResult) GetAllocationForCoinType(coinType dcrutil.CoinType) *CoinTypeAllocation {
	return result.Allocations[coinType]
}

// GetUtilizationPercentage returns the overall block space utilization as a percentage.
func (result *AllocationResult) GetUtilizationPercentage() float64 {
	if result.TotalAllocated == 0 {
		return 0.0
	}
	return (float64(result.TotalUsed) / float64(result.TotalAllocated)) * 100.0
}

// Helper function to return the minimum of two uint32 values.
func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// GetTransactionCoinType determines the primary coin type of a transaction
// based on the majority of its outputs.
func GetTransactionCoinType(tx *dcrutil.Tx) dcrutil.CoinType {
	msgTx := tx.MsgTx()
	if len(msgTx.TxOut) == 0 {
		return dcrutil.CoinTypeVAR // Default to VAR for transactions with no outputs
	}

	// Count outputs by coin type
	coinTypeCounts := make(map[wire.CoinType]int)
	for _, txOut := range msgTx.TxOut {
		coinTypeCounts[txOut.CoinType]++
	}

	// Find the coin type with the most outputs
	var primaryCoinType wire.CoinType = wire.CoinTypeVAR
	maxCount := 0

	for coinType, count := range coinTypeCounts {
		if count > maxCount {
			maxCount = count
			primaryCoinType = coinType
		}
	}

	return dcrutil.CoinType(primaryCoinType)
}

// TransactionSizeTracker tracks transaction sizes by coin type for block space allocation.
type TransactionSizeTracker struct {
	sizesByCoinType map[dcrutil.CoinType]uint32
	allocator       *BlockSpaceAllocator
}

// NewTransactionSizeTracker creates a new transaction size tracker.
func NewTransactionSizeTracker(allocator *BlockSpaceAllocator) *TransactionSizeTracker {
	return &TransactionSizeTracker{
		sizesByCoinType: make(map[dcrutil.CoinType]uint32),
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
	testSizes := make(map[dcrutil.CoinType]uint32)
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
func (tst *TransactionSizeTracker) GetSizeForCoinType(coinType dcrutil.CoinType) uint32 {
	return tst.sizesByCoinType[coinType]
}

// Reset clears all tracked transaction sizes.
func (tst *TransactionSizeTracker) Reset() {
	tst.sizesByCoinType = make(map[dcrutil.CoinType]uint32)
}
