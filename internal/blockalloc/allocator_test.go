// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
	"fmt"
	"testing"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
)

// mockChainParams creates a test chain parameters with configured SKA types.
func mockChainParams() *chaincfg.Params {
	params := &chaincfg.Params{}

	// Configure 3 SKA types for testing
	params.SKACoins = map[cointype.CoinType]*chaincfg.SKACoinConfig{
		1: {
			CoinType:       1,
			Name:           "Skarb-1",
			Symbol:         "SKA-1",
			EmissionHeight: 100,
			EmissionWindow: 50,
			Active:         true,
		},
		2: {
			CoinType:       2,
			Name:           "Skarb-2",
			Symbol:         "SKA-2",
			EmissionHeight: 200,
			EmissionWindow: 50,
			Active:         true,
		},
		3: {
			CoinType:       3,
			Name:           "Skarb-3",
			Symbol:         "SKA-3",
			EmissionHeight: 300,
			EmissionWindow: 50,
			Active:         false, // Inactive for testing
		},
	}

	return params
}

// TestNewBlockSpaceAllocator verifies basic allocator construction.
func TestNewBlockSpaceAllocator(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	if allocator.maxBlockSize != 1000000 {
		t.Errorf("Expected maxBlockSize 1000000, got %d", allocator.maxBlockSize)
	}

	if allocator.varAllocation != 0.10 {
		t.Errorf("Expected varAllocation 0.10, got %f", allocator.varAllocation)
	}

	if allocator.skaAllocation != 0.90 {
		t.Errorf("Expected skaAllocation 0.90, got %f", allocator.skaAllocation)
	}
}

// TestBaseAllocations verifies the base 10%/90% allocation calculation.
func TestBaseAllocations(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	baseAllocations := allocator.calculateBaseAllocations()

	// VAR should get 10% = 100KB
	varAllocation := baseAllocations[cointype.CoinTypeVAR]
	if varAllocation != 100000 {
		t.Errorf("Expected VAR allocation 100000, got %d", varAllocation)
	}

	// SKA-1 should get 45% = 450KB (90% / 2 active SKA types)
	ska1Allocation := baseAllocations[1]
	if ska1Allocation != 450000 {
		t.Errorf("Expected SKA-1 allocation 450000, got %d", ska1Allocation)
	}

	// SKA-2 should get 45% = 450KB
	ska2Allocation := baseAllocations[2]
	if ska2Allocation != 450000 {
		t.Errorf("Expected SKA-2 allocation 450000, got %d", ska2Allocation)
	}

	// SKA-3 should not be allocated (inactive)
	if _, exists := baseAllocations[3]; exists {
		t.Error("SKA-3 should not have allocation (inactive)")
	}
}

// TestHighDemandScenario tests the complex example: VAR=800KB, SKA-1=1000KB, SKA-2=100KB.
func TestHighDemandScenario(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 800000,  // 800KB pending
		1:                    1000000, // 1000KB pending (SKA-1)
		2:                    100000,  // 100KB pending (SKA-2)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// Verify VAR allocation: 100KB base + 35KB overflow = 135KB
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.BaseAllocation != 100000 {
		t.Errorf("Expected VAR base allocation 100000, got %d", varAlloc.BaseAllocation)
	}
	if varAlloc.UsedBytes != 135000 {
		t.Errorf("Expected VAR used bytes 135000, got %d", varAlloc.UsedBytes)
	}

	// Verify SKA-1 allocation: 450KB base + 315KB overflow = 765KB
	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.BaseAllocation != 450000 {
		t.Errorf("Expected SKA-1 base allocation 450000, got %d", ska1Alloc.BaseAllocation)
	}
	if ska1Alloc.UsedBytes != 765000 {
		t.Errorf("Expected SKA-1 used bytes 765000, got %d", ska1Alloc.UsedBytes)
	}

	// Verify SKA-2 allocation: 100KB used (no overflow needed)
	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc.UsedBytes != 100000 {
		t.Errorf("Expected SKA-2 used bytes 100000, got %d", ska2Alloc.UsedBytes)
	}

	// Verify total utilization
	expectedTotal := uint32(135000 + 765000 + 100000) // 1000KB = 100% utilization
	if result.TotalUsed != expectedTotal {
		t.Errorf("Expected total used %d, got %d", expectedTotal, result.TotalUsed)
	}
}

// TestNoVARDemandScenario tests when VAR has no pending transactions.
func TestNoVARDemandScenario(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 0,      // No VAR demand
		1:                    500000, // 500KB pending (SKA-1)
		2:                    300000, // 300KB pending (SKA-2)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should use 0 bytes
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.UsedBytes != 0 {
		t.Errorf("Expected VAR used bytes 0, got %d", varAlloc.UsedBytes)
	}

	// SKA-1 should get its base + some overflow
	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.UsedBytes != 500000 { // Uses all its pending (450KB base + 50KB more)
		t.Errorf("Expected SKA-1 used bytes 500000, got %d", ska1Alloc.UsedBytes)
	}

	// SKA-2 should use all its pending
	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc.UsedBytes != 300000 {
		t.Errorf("Expected SKA-2 used bytes 300000, got %d", ska2Alloc.UsedBytes)
	}
}

// TestOnlyVARDemandScenario tests when only VAR has pending transactions.
func TestOnlyVARDemandScenario(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 800000, // 800KB pending
		1:                    0,      // No SKA-1 demand
		2:                    0,      // No SKA-2 demand
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should get 100% of overflow since it's the only one with pending
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	// Base 100KB + all 900KB overflow = but VAR only needs 800KB total
	expectedVARUsage := uint32(800000) // Uses what it needs
	if varAlloc.UsedBytes != expectedVARUsage {
		t.Errorf("Expected VAR used bytes %d, got %d", expectedVARUsage, varAlloc.UsedBytes)
	}

	// SKA types should use 0 bytes
	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.UsedBytes != 0 {
		t.Errorf("Expected SKA-1 used bytes 0, got %d", ska1Alloc.UsedBytes)
	}

	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc.UsedBytes != 0 {
		t.Errorf("Expected SKA-2 used bytes 0, got %d", ska2Alloc.UsedBytes)
	}
}

// TestMultipleSKAWithDemand tests overflow distribution among multiple SKA types.
func TestMultipleSKAWithDemand(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 200000, // 200KB pending
		1:                    800000, // 800KB pending (SKA-1)
		2:                    700000, // 700KB pending (SKA-2)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// Calculate expected overflow distribution
	// Base: VAR=100KB, SKA-1=450KB, SKA-2=450KB
	// Usage: VAR=100KB, SKA-1=450KB, SKA-2=450KB
	// No overflow since everyone uses their full base allocation
	// Wait, let me recalculate:
	// Total pending: 200+800+700=1700KB, Total space: 1000KB
	// Base allocations: VAR=100KB, SKA-1=450KB, SKA-2=450KB
	// After base: VAR uses 100KB (100KB still pending), SKA-1 uses 450KB (350KB pending), SKA-2 uses 450KB (250KB pending)
	// No overflow since all base allocations are fully used

	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.UsedBytes != 100000 {
		t.Errorf("Expected VAR used bytes 100000, got %d", varAlloc.UsedBytes)
	}

	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.UsedBytes != 450000 {
		t.Errorf("Expected SKA-1 used bytes 450000, got %d", ska1Alloc.UsedBytes)
	}

	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc.UsedBytes != 450000 {
		t.Errorf("Expected SKA-2 used bytes 450000, got %d", ska2Alloc.UsedBytes)
	}

	// Total should be exactly 1MB since all base allocations are used
	if result.TotalUsed != 1000000 {
		t.Errorf("Expected total used 1000000, got %d", result.TotalUsed)
	}
}

// TestIdentifyActivePendingTypes verifies active pending type detection.
func TestIdentifyActivePendingTypes(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params)

	// Setup scenario where VAR and SKA-1 have pending after base allocation
	allocations := map[cointype.CoinType]*CoinTypeAllocation{
		cointype.CoinTypeVAR: {
			CoinType:     cointype.CoinTypeVAR,
			PendingBytes: 800000,
			UsedBytes:    100000, // 700KB still pending
		},
		1: {
			CoinType:     1,
			PendingBytes: 1000000,
			UsedBytes:    450000, // 550KB still pending
		},
		2: {
			CoinType:     2,
			PendingBytes: 100000,
			UsedBytes:    100000, // 0KB pending
		},
	}

	activePending := allocator.identifyActivePendingTypes(allocations)

	// Should identify VAR and SKA-1 as having pending
	if len(activePending) != 2 {
		t.Errorf("Expected 2 active pending types, got %d", len(activePending))
	}

	// Verify specific types are included
	hasVAR := false
	hasSKA1 := false
	for _, coinType := range activePending {
		if coinType == cointype.CoinTypeVAR {
			hasVAR = true
		}
		if coinType == 1 {
			hasSKA1 = true
		}
	}

	if !hasVAR {
		t.Error("Expected VAR to be in active pending types")
	}
	if !hasSKA1 {
		t.Error("Expected SKA-1 to be in active pending types")
	}
}

// TestUtilizationPercentage verifies utilization calculation.
func TestUtilizationPercentage(t *testing.T) {
	result := &AllocationResult{
		TotalAllocated: 1000000, // 1MB
		TotalUsed:      750000,  // 750KB
	}

	utilization := result.GetUtilizationPercentage()
	expected := 75.0 // 75%

	if utilization != expected {
		t.Errorf("Expected utilization %f%%, got %f%%", expected, utilization)
	}
}

// TestEdgeCaseZeroPending tests behavior with no pending transactions.
func TestEdgeCaseZeroPending(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params)

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 0, // No pending
		1:                    0, // No pending
		2:                    0, // No pending
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// All allocations should use 0 bytes
	for coinType, allocation := range result.Allocations {
		if allocation.UsedBytes != 0 {
			t.Errorf("Expected coin type %d to use 0 bytes, got %d", coinType, allocation.UsedBytes)
		}
	}

	// Total utilization should be 0%
	if result.GetUtilizationPercentage() != 0.0 {
		t.Errorf("Expected 0%% utilization, got %f%%", result.GetUtilizationPercentage())
	}
}

// TestNoSKATypesScenario tests the critical case where no SKA types are active.
// This verifies that VAR can claim all block space when it's the only pending type.
func TestNoSKATypesScenario(t *testing.T) {
	// Create params with no active SKA types
	params := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{},
	}

	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 950000, // 950KB pending (more than 10% base)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should be able to use up to the full block size
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc == nil {
		t.Fatal("Expected VAR allocation to exist")
	}

	// VAR base should be 10% = 100KB
	if varAlloc.BaseAllocation != 100000 {
		t.Errorf("Expected VAR base allocation 100000, got %d", varAlloc.BaseAllocation)
	}

	// VAR should use all its pending (950KB)
	if varAlloc.UsedBytes != 950000 {
		t.Errorf("Expected VAR to use 950000 bytes, got %d", varAlloc.UsedBytes)
	}

	// Final allocation should accommodate all usage
	if varAlloc.FinalAllocation < 950000 {
		t.Errorf("Expected VAR final allocation >= 950000, got %d", varAlloc.FinalAllocation)
	}
}

// TestIntegerMathPrecision verifies that integer math preserves all bytes.
func TestIntegerMathPrecision(t *testing.T) {
	testCases := []uint32{
		1000000, // 1MB - divides evenly
		999999,  // 1MB - 1 byte
		1000001, // 1MB + 1 byte
		123456,  // Arbitrary size
	}

	for _, blockSize := range testCases {
		t.Run(fmt.Sprintf("BlockSize=%d", blockSize), func(t *testing.T) {
			params := mockChainParams()
			allocator := NewBlockSpaceAllocator(blockSize, params)

			baseAllocations := allocator.calculateBaseAllocations()

			// Sum all base allocations
			totalAllocated := uint32(0)
			for _, allocation := range baseAllocations {
				totalAllocated += allocation
			}

			// Total should equal block size exactly
			if totalAllocated != blockSize {
				t.Errorf("Total base allocations %d != block size %d, lost %d bytes",
					totalAllocated, blockSize, blockSize-totalAllocated)
			}
		})
	}
}

// TestIterativeOverflowDistribution tests that overflow is fully distributed.
func TestIterativeOverflowDistribution(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB

	// Scenario: VAR needs 200KB (100KB base + 100KB overflow)
	//          SKA-1 needs 300KB (can't fit in 450KB base)
	//          SKA-2 needs 600KB (150KB over base)
	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 200000, // 200KB
		1:                    300000, // 300KB - fits in base
		2:                    600000, // 600KB - needs overflow
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// Verify all pending demand is satisfied up to block limit
	totalDemand := uint32(200000 + 300000 + 600000) // 1100KB
	expectedUsage := min(totalDemand, 1000000)      // Capped at 1MB

	if result.TotalUsed != expectedUsage {
		t.Errorf("Expected total usage %d, got %d", expectedUsage, result.TotalUsed)
	}

	// Verify proportional distribution within limits
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	ska1Alloc := result.GetAllocationForCoinType(1)
	ska2Alloc := result.GetAllocationForCoinType(2)

	// All allocations should exist
	if varAlloc == nil || ska1Alloc == nil || ska2Alloc == nil {
		t.Fatal("Missing allocations")
	}

	// SKA-1 should get its full 300KB (fits in base)
	if ska1Alloc.UsedBytes != 300000 {
		t.Errorf("Expected SKA-1 to use 300000, got %d", ska1Alloc.UsedBytes)
	}
}

// TestMixedPendingWithNewCoinType tests handling of coin types not in base allocation.
func TestMixedPendingWithNewCoinType(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params)

	// Add pending for a coin type that doesn't have base allocation
	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 50000,  // 50KB
		1:                    100000, // 100KB
		2:                    100000, // 100KB
		99:                   200000, // 200KB - new coin type not in base!
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// Coin type 99 should get an allocation entry even though it had no base
	newTypeAlloc := result.GetAllocationForCoinType(99)
	if newTypeAlloc == nil {
		t.Fatal("Expected allocation for new coin type 99")
	}

	// It should have zero base but can claim overflow
	if newTypeAlloc.BaseAllocation != 0 {
		t.Errorf("Expected zero base for type 99, got %d", newTypeAlloc.BaseAllocation)
	}

	// Since other types don't use all their base, type 99 should get some space
	if newTypeAlloc.UsedBytes == 0 {
		t.Error("Expected new coin type to get some overflow space")
	}
}

// TestMinFunction verifies the min utility function.
func TestMinFunction(t *testing.T) {
	testCases := []struct {
		a, b, expected uint32
	}{
		{10, 20, 10},
		{30, 15, 15},
		{100, 100, 100},
		{0, 50, 0},
	}

	for _, tc := range testCases {
		result := min(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tc.a, tc.b, result, tc.expected)
		}
	}
}
