// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"sort"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
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
func (bsa *BlockSpaceAllocator) AllocateBlockSpace(pendingTxBytes map[cointype.CoinType]uint32) *AllocationResult {
	// Phase 1: Calculate base allocations
	baseAllocations := bsa.calculateBaseAllocations()

	// Phase 2: Initialize allocations for ALL pending coin types
	allocations := make(map[cointype.CoinType]*CoinTypeAllocation)

	// First, create entries for coin types with base allocations
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
	}

	// Ensure every coin type with pending transactions has an allocation entry
	for coinType, pending := range pendingTxBytes {
		if _, ok := allocations[coinType]; !ok && pending > 0 {
			allocations[coinType] = &CoinTypeAllocation{
				CoinType:        coinType,
				BaseAllocation:  0,
				FinalAllocation: 0,
				PendingBytes:    pending,
				UsedBytes:       0,
			}
		}
	}

	// Calculate total overflow including unallocated space and unused base allocations
	sumBase := uint32(0)
	for _, baseSpace := range baseAllocations {
		sumBase += baseSpace
	}
	roundingRemainder := bsa.maxBlockSize - sumBase
	totalOverflow := roundingRemainder

	// Add unused base allocations to overflow
	for _, allocation := range allocations {
		if allocation.UsedBytes < allocation.BaseAllocation {
			totalOverflow += allocation.BaseAllocation - allocation.UsedBytes
		}
	}

	// Phase 3: Identify coin types with remaining demand
	activePendingTypes := bsa.identifyActivePendingTypes(allocations)

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
func (bsa *BlockSpaceAllocator) calculateBaseAllocations() map[cointype.CoinType]uint32 {
	allocations := make(map[cointype.CoinType]uint32)

	// Use integer math to avoid rounding losses
	// VAR gets exactly 10% using integer division
	varSpace := bsa.maxBlockSize / 10 // Exactly 10%
	allocations[cointype.CoinTypeVAR] = varSpace

	// SKA types share the remaining 90%
	skaSpace := bsa.maxBlockSize - varSpace // Exactly the rest

	activeSKATypes := bsa.chainParams.GetActiveSKATypes()
	if len(activeSKATypes) > 0 {
		// Distribute SKA space equally with remainder handling
		skaSpacePerType := skaSpace / uint32(len(activeSKATypes))
		remainder := skaSpace % uint32(len(activeSKATypes))

		// Sort SKA types for deterministic remainder distribution
		sortedSKATypes := make([]cointype.CoinType, len(activeSKATypes))
		copy(sortedSKATypes, activeSKATypes)
		sort.Slice(sortedSKATypes, func(i, j int) bool {
			return sortedSKATypes[i] < sortedSKATypes[j]
		})

		for i, skaType := range sortedSKATypes {
			allocation := skaSpacePerType
			// Distribute remainder bytes to first few types
			if uint32(i) < remainder {
				allocation++
			}
			allocations[skaType] = allocation
		}
	}
	// Note: If no SKA types are active, the 90% space will be added to overflow
	// in AllocateBlockSpace and can be claimed by any pending coin type

	return allocations
}

// identifyActivePendingTypes finds coin types that have pending transactions
// after their base allocation is filled.
func (bsa *BlockSpaceAllocator) identifyActivePendingTypes(
	allocations map[cointype.CoinType]*CoinTypeAllocation) []cointype.CoinType {

	var activePending []cointype.CoinType

	for coinType, allocation := range allocations {
		remainingDemand := allocation.PendingBytes - allocation.UsedBytes
		if remainingDemand > 0 {
			activePending = append(activePending, coinType)
		}
	}

	return activePending
}

// distributeOverflow distributes unused block space among coin types with pending
// transactions using iterative proportional distribution within 10%/90% buckets.
func (bsa *BlockSpaceAllocator) distributeOverflow(
	allocations map[cointype.CoinType]*CoinTypeAllocation,
	totalOverflow uint32,
	activePendingTypes []cointype.CoinType) {

	remaining := totalOverflow
	maxIterations := 10 // Prevent infinite loops

	for iteration := 0; iteration < maxIterations && remaining > 0; iteration++ {
		// Partition active pending types and calculate demands
		var varTypes, skaTypes []cointype.CoinType
		var varDemand, skaDemand uint64

		for _, coinType := range activePendingTypes {
			allocation := allocations[coinType]
			demand := uint64(allocation.PendingBytes - allocation.UsedBytes)
			if demand == 0 {
				continue
			}

			if coinType == cointype.CoinTypeVAR {
				varTypes = append(varTypes, coinType)
				varDemand += demand
			} else {
				skaTypes = append(skaTypes, coinType)
				skaDemand += demand
			}
		}

		// If no demand remains, stop
		if varDemand+skaDemand == 0 {
			break
		}

		// Sort coin types for deterministic distribution
		sort.Slice(varTypes, func(i, j int) bool { return varTypes[i] < varTypes[j] })
		sort.Slice(skaTypes, func(i, j int) bool { return skaTypes[i] < skaTypes[j] })

		// Calculate bucket shares based on configured VAR/SKA split
		var varShare, skaShare uint64
		if len(varTypes) > 0 && len(skaTypes) > 0 {
			// Both have demand - use configured split
			varShare = uint64(float64(remaining) * bsa.varAllocation)
			skaShare = uint64(remaining) - varShare // Remainder goes to SKA
		} else if len(varTypes) > 0 {
			// Only VAR has demand - gets everything
			varShare = uint64(remaining)
			skaShare = 0
		} else if len(skaTypes) > 0 {
			// Only SKA has demand - gets everything
			varShare = 0
			skaShare = uint64(remaining)
		}

		// Cap shares by actual demand
		if varShare > varDemand {
			varShare = varDemand
		}
		if skaShare > skaDemand {
			skaShare = skaDemand
		}

		// Distribute proportionally within each bucket
		consumed := uint32(0)

		// Helper function to distribute within a bucket proportionally
		distributeToBucket := func(types []cointype.CoinType, bucketShare, bucketDemand uint64) {
			if bucketShare == 0 || bucketDemand == 0 || len(types) == 0 {
				return
			}

			leftover := bucketShare
			for i, coinType := range types {
				allocation := allocations[coinType]
				demand := uint64(allocation.PendingBytes - allocation.UsedBytes)

				var give uint64
				if i == len(types)-1 {
					// Last type gets all remaining to avoid rounding loss
					give = leftover
				} else {
					// Proportional distribution
					give = (bucketShare * demand) / bucketDemand
				}

				// Cap by actual demand
				if give > demand {
					give = demand
				}

				if give > 0 {
					add := uint32(give)
					allocation.UsedBytes += add
					allocation.FinalAllocation += add
					consumed += add
					leftover -= give
				}
			}
		}

		distributeToBucket(varTypes, varShare, varDemand)
		distributeToBucket(skaTypes, skaShare, skaDemand)

		// If nothing was consumed, we can't make progress
		if consumed == 0 {
			break
		}

		remaining -= consumed
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
