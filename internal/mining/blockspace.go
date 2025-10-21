// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/internal/blockalloc"
	"github.com/decred/dcrd/internal/fees"
)

// BlockSpaceAllocator extends the core blockalloc.BlockSpaceAllocator with
// fee calculator integration for mining-specific functionality.
type BlockSpaceAllocator struct {
	*blockalloc.BlockSpaceAllocator
	feeCalculator *fees.CoinTypeFeeCalculator
}

// NewBlockSpaceAllocator creates a new block space allocator with the standard
// 10% VAR / 90% SKA allocation strategy.
func NewBlockSpaceAllocator(maxBlockSize uint32, chainParams *chaincfg.Params) *BlockSpaceAllocator {
	return &BlockSpaceAllocator{
		BlockSpaceAllocator: blockalloc.NewBlockSpaceAllocator(maxBlockSize, chainParams),
		feeCalculator:       nil, // Set by SetFeeCalculator
	}
}

// NewBlockSpaceAllocatorWithFeeCalculator creates a new block space allocator with integrated fee calculator.
func NewBlockSpaceAllocatorWithFeeCalculator(maxBlockSize uint32, chainParams *chaincfg.Params,
	feeCalculator *fees.CoinTypeFeeCalculator) *BlockSpaceAllocator {
	return &BlockSpaceAllocator{
		BlockSpaceAllocator: blockalloc.NewBlockSpaceAllocator(maxBlockSize, chainParams),
		feeCalculator:       feeCalculator,
	}
}

// SetFeeCalculator sets the fee calculator for utilization tracking.
func (bsa *BlockSpaceAllocator) SetFeeCalculator(feeCalculator *fees.CoinTypeFeeCalculator) {
	bsa.feeCalculator = feeCalculator
}

// AllocateBlockSpace calculates the optimal block space allocation and updates
// fee calculator utilization stats if available.
func (bsa *BlockSpaceAllocator) AllocateBlockSpace(pendingTxBytes map[cointype.CoinType]uint32) *blockalloc.AllocationResult {
	result := bsa.BlockSpaceAllocator.AllocateBlockSpace(pendingTxBytes)

	// Update fee calculator with utilization stats if available
	if bsa.feeCalculator != nil {
		bsa.updateFeeCalculatorUtilization(result.Allocations, pendingTxBytes)
	}

	return result
}

// updateFeeCalculatorUtilization updates the fee calculator with current network utilization stats.
func (bsa *BlockSpaceAllocator) updateFeeCalculatorUtilization(allocations map[cointype.CoinType]*blockalloc.CoinTypeAllocation,
	pendingTxBytes map[cointype.CoinType]uint32) {

	// Count pending transactions (estimate based on average transaction size)
	const avgTxSize = 250 // Average transaction size in bytes

	for coinType, allocation := range allocations {
		pending := pendingTxBytes[coinType]
		pendingTxCount := int(pending / avgTxSize) // Rough estimate

		// Calculate block space utilization for this coin type
		var blockSpaceUsed float64
		if allocation.FinalAllocation > 0 {
			blockSpaceUsed = float64(allocation.UsedBytes) / float64(allocation.FinalAllocation)
		}

		// Update fee calculator with utilization data
		bsa.feeCalculator.UpdateUtilization(coinType, pendingTxCount,
			int64(pending), blockSpaceUsed)
	}
}

// RecordTransactionFee records a transaction fee for fee estimation (called from mining integration).
func (bsa *BlockSpaceAllocator) RecordTransactionFee(coinType cointype.CoinType, fee int64, size int64, confirmed bool) {
	if bsa.feeCalculator != nil {
		bsa.feeCalculator.RecordTransactionFee(coinType, fee, size, confirmed)
	}
}

// ValidateTransactionFees validates fees for a transaction using the integrated fee calculator.
func (bsa *BlockSpaceAllocator) ValidateTransactionFees(txFee int64, serializedSize int64,
	coinType cointype.CoinType, allowHighFees bool) error {
	if bsa.feeCalculator != nil {
		return bsa.feeCalculator.ValidateTransactionFees(txFee, serializedSize, coinType, allowHighFees)
	}
	// Fall back to basic validation if no fee calculator
	return nil
}

// GetFeeEstimate returns fee estimate for a coin type and target confirmations.
func (bsa *BlockSpaceAllocator) GetFeeEstimate(coinType cointype.CoinType, targetConfirmations int) (dcrutil.Amount, error) {
	if bsa.feeCalculator != nil {
		return bsa.feeCalculator.EstimateFeeRate(coinType, targetConfirmations)
	}
	// Return basic estimate if no fee calculator
	return dcrutil.Amount(1e4), nil // Default 10000 atoms/KB
}
