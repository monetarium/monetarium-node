// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/slog"
)

var log = slog.Disabled

// UseLogger uses a specified Logger to output package logging info.
func UseLogger(logger slog.Logger) {
	log = logger
}

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
	CoinType        cointype.CoinType
	BaseAllocation  uint32 // Guaranteed space allocation
	FinalAllocation uint32 // Final space after overflow distribution
	PendingBytes    uint32 // Bytes of transactions pending for this coin type
	UsedBytes       uint32 // Bytes actually used by this coin type
}

// AllocationResult contains the complete block space allocation for all coin types.
type AllocationResult struct {
	Allocations     map[cointype.CoinType]*CoinTypeAllocation
	TotalAllocated  uint32
	TotalUsed       uint32
	OverflowHandled uint32
}

// AllocateBlockSpace calculates the optimal block space allocation given pending
// transaction sizes for each coin type. Returns allocation details for all coin types.
//
// Algorithm:
// 1. If no SKA has pending transactions, VAR gets 100% of block space (early exit)
// 2. Otherwise, initial 10% VAR / 90% SKA split among active SKA types
// 3. Redistribute unused space ONCE with 10%/90% proportional allocation
// 4. Any remaining unused space goes to VAR
func (bsa *BlockSpaceAllocator) AllocateBlockSpace(pendingTxBytes map[cointype.CoinType]uint32) *AllocationResult {
	allocations := make(map[cointype.CoinType]*CoinTypeAllocation)
	activeSKATypes := bsa.chainParams.GetActiveSKATypes()

	// Initialize default allocations for all active coin types (prevents nil pointer issues)
	varPending := pendingTxBytes[cointype.CoinTypeVAR]
	allocations[cointype.CoinTypeVAR] = &CoinTypeAllocation{
		CoinType:        cointype.CoinTypeVAR,
		BaseAllocation:  0,
		FinalAllocation: 0,
		PendingBytes:    varPending,
		UsedBytes:       0,
	}

	for _, skaType := range activeSKATypes {
		allocations[skaType] = &CoinTypeAllocation{
			CoinType:        skaType,
			BaseAllocation:  0,
			FinalAllocation: 0,
			PendingBytes:    pendingTxBytes[skaType],
			UsedBytes:       0,
		}
	}

	// Step 1: Check if any SKA types have pending transactions
	hasSKAPending := false
	for coinType, pending := range pendingTxBytes {
		if coinType.IsSKA() && pending > 0 {
			hasSKAPending = true
			break
		}
	}

	// Early exit: No SKA pending, VAR gets entire block
	if !hasSKAPending {
		allocations[cointype.CoinTypeVAR].BaseAllocation = bsa.maxBlockSize
		allocations[cointype.CoinTypeVAR].FinalAllocation = bsa.maxBlockSize
		allocations[cointype.CoinTypeVAR].UsedBytes = min(varPending, bsa.maxBlockSize)

		return &AllocationResult{
			Allocations:    allocations,
			TotalAllocated: bsa.maxBlockSize,
			TotalUsed:      allocations[cointype.CoinTypeVAR].UsedBytes,
		}
	}

	// Step 2: Initial 10%/90% split
	varBase := bsa.maxBlockSize / 10
	skaBase := bsa.maxBlockSize - varBase

	varUsed := min(varPending, varBase)
	varUnused := varBase - varUsed

	allocations[cointype.CoinTypeVAR].BaseAllocation = varBase
	allocations[cointype.CoinTypeVAR].FinalAllocation = varBase
	allocations[cointype.CoinTypeVAR].UsedBytes = varUsed

	skaPerType := skaBase / uint32(len(activeSKATypes))

	totalSKAUnused := uint32(0)
	for _, skaType := range activeSKATypes {
		skaPending := pendingTxBytes[skaType]
		skaUsed := min(skaPending, skaPerType)
		totalSKAUnused += skaPerType - skaUsed

		allocations[skaType].BaseAllocation = skaPerType
		allocations[skaType].FinalAllocation = skaPerType
		allocations[skaType].UsedBytes = skaUsed
	}

	// Step 3: Single redistribution of unused space
	totalUnused := varUnused + totalSKAUnused

	if totalUnused > 0 {
		// Calculate remaining needs for each coin type
		varNeed := int64(varPending) - int64(varUsed)
		if varNeed < 0 {
			varNeed = 0
		}

		skaNeeds := make(map[cointype.CoinType]int64)
		totalSKANeed := int64(0)
		for _, skaType := range activeSKATypes {
			alloc := allocations[skaType]
			need := int64(alloc.PendingBytes) - int64(alloc.UsedBytes)
			if need > 0 {
				skaNeeds[skaType] = need
				totalSKANeed += need
			}
		}

		// Distribute unused with 10%/90% split
		varShare := uint32(float64(totalUnused) * 0.10)
		skaShare := totalUnused - varShare

		// Give to VAR (capped by need)
		varGets := min(uint32(varNeed), varShare)
		if varGets > 0 {
			allocations[cointype.CoinTypeVAR].FinalAllocation += varGets
			allocations[cointype.CoinTypeVAR].UsedBytes += varGets
		}
		varLeftover := varShare - varGets

		// Give to SKA types proportionally by need
		skaUsedFromShare := uint32(0)
		if totalSKANeed > 0 {
			for _, skaType := range activeSKATypes {
				need := skaNeeds[skaType]
				if need > 0 {
					proportion := float64(need) / float64(totalSKANeed)
					skaGets := uint32(float64(skaShare) * proportion)
					skaGets = min(skaGets, uint32(need))

					if skaGets > 0 {
						allocations[skaType].FinalAllocation += skaGets
						allocations[skaType].UsedBytes += skaGets
						skaUsedFromShare += skaGets
					}
				}
			}
		}
		skaLeftover := skaShare - skaUsedFromShare

		// Step 4: All remaining unused goes to VAR
		totalLeftover := varLeftover + skaLeftover
		if totalLeftover > 0 {
			allocations[cointype.CoinTypeVAR].FinalAllocation += totalLeftover
		}
	}

	// Calculate totals
	totalUsed := uint32(0)
	totalAllocated := uint32(0)
	for _, alloc := range allocations {
		totalUsed += alloc.UsedBytes
		totalAllocated += alloc.FinalAllocation
	}

	// Sanity check with warning (should never happen with correct logic)
	if totalAllocated > bsa.maxBlockSize {
		log.Warnf("Block space allocation overflow: total=%d exceeds max=%d (diff=%d)",
			totalAllocated, bsa.maxBlockSize, totalAllocated-bsa.maxBlockSize)
		// Cap to prevent issues
		totalAllocated = bsa.maxBlockSize
	}

	return &AllocationResult{
		Allocations:    allocations,
		TotalAllocated: totalAllocated,
		TotalUsed:      totalUsed,
	}
}

// GetAllocationForCoinType returns the space allocation for a specific coin type.
func (result *AllocationResult) GetAllocationForCoinType(coinType cointype.CoinType) *CoinTypeAllocation {
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
