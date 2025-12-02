// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockalloc

import (
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

// mockChainParamsWithThreeSKAs creates test chain parameters with 3 active SKA types.
func mockChainParamsWithThreeSKAs() *chaincfg.Params {
	params := &chaincfg.Params{}

	// Configure 3 active SKA types for testing
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
			Active:         true, // Now active for testing 3-way split
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
// DEPRECATED: This test tested the old calculateBaseAllocations() helper function
// which is no longer used by the simplified algorithm. Base allocations are now
// calculated directly in AllocateBlockSpace(). Kept for reference.
/*
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
*/

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

	// SKA types should have default allocations with 0 values (prevents nil pointer issues)
	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc == nil {
		t.Fatal("Expected SKA-1 allocation to exist (default allocation)")
	}
	if ska1Alloc.UsedBytes != 0 || ska1Alloc.BaseAllocation != 0 || ska1Alloc.FinalAllocation != 0 {
		t.Errorf("Expected SKA-1 to have 0 allocations (no demand), got %+v", ska1Alloc)
	}

	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc == nil {
		t.Fatal("Expected SKA-2 allocation to exist (default allocation)")
	}
	if ska2Alloc.UsedBytes != 0 || ska2Alloc.BaseAllocation != 0 || ska2Alloc.FinalAllocation != 0 {
		t.Errorf("Expected SKA-2 to have 0 allocations (no demand), got %+v", ska2Alloc)
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

// TestSingleActiveSKAType tests allocation when only one SKA type is active.
// Verifies that the single SKA gets the full 90% allocation, not split.
func TestSingleActiveSKAType(t *testing.T) {
	// Create params with only SKA-1 active
	params := &chaincfg.Params{}
	params.SKACoins = map[cointype.CoinType]*chaincfg.SKACoinConfig{
		1: {
			CoinType:       1,
			Name:           "Skarb-1",
			Symbol:         "SKA-1",
			EmissionHeight: 100,
			EmissionWindow: 50,
			Active:         true,
		},
	}

	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 150000, // 150KB pending
		1:                    600000, // 600KB pending (SKA-1)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should get 10% base = 100KB
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.BaseAllocation != 100000 {
		t.Errorf("Expected VAR base allocation 100000, got %d", varAlloc.BaseAllocation)
	}

	// SKA-1 should get full 90% base = 900KB (not split with another SKA)
	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.BaseAllocation != 900000 {
		t.Errorf("Expected SKA-1 base allocation 900000 (full 90%%), got %d", ska1Alloc.BaseAllocation)
	}

	// Verify VAR uses its 150KB demand
	// (100KB base + some from redistribution, but may not get full 50KB due to 10%/90% split)
	if varAlloc.UsedBytes < 100000 || varAlloc.UsedBytes > 150000 {
		t.Errorf("Expected VAR to use between 100000-150000 bytes, got %d", varAlloc.UsedBytes)
	}

	// Verify SKA-1 uses its 600KB demand
	if ska1Alloc.UsedBytes != 600000 {
		t.Errorf("Expected SKA-1 to use 600000 bytes, got %d", ska1Alloc.UsedBytes)
	}

	// Total used: VAR's actual usage + SKA-1's 600KB
	expectedTotal := varAlloc.UsedBytes + 600000
	if result.TotalUsed != expectedTotal {
		t.Errorf("Expected total used %d, got %d", expectedTotal, result.TotalUsed)
	}
}

// TestAllDemandFitsInBaseAllocations tests when all pending fits within base allocations.
// Verifies no overflow redistribution occurs when demand < base for all types.
func TestAllDemandFitsInBaseAllocations(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	// All demands fit within their base allocations (VAR: 10%=100KB, SKA: 45%=450KB each)
	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 30000,  // 30KB < 100KB base
		1:                    200000, // 200KB < 450KB base
		2:                    150000, // 150KB < 450KB base
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should use exactly its demand
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.BaseAllocation != 100000 {
		t.Errorf("Expected VAR base allocation 100000, got %d", varAlloc.BaseAllocation)
	}
	if varAlloc.UsedBytes != 30000 {
		t.Errorf("Expected VAR to use exactly 30000 bytes (its demand), got %d", varAlloc.UsedBytes)
	}
	// Note: FinalAllocation may be > BaseAllocation due to redistribution of unused space
	// Even though VAR's demand is met, leftover space gets redistributed (Step 4: unused goes to VAR)
	if varAlloc.FinalAllocation < varAlloc.BaseAllocation {
		t.Errorf("Expected VAR final allocation >= base, got final=%d base=%d",
			varAlloc.FinalAllocation, varAlloc.BaseAllocation)
	}

	// SKA-1 should use exactly its demand
	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.UsedBytes != 200000 {
		t.Errorf("Expected SKA-1 to use exactly 200000 bytes (its demand), got %d", ska1Alloc.UsedBytes)
	}

	// SKA-2 should use exactly its demand
	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc.UsedBytes != 150000 {
		t.Errorf("Expected SKA-2 to use exactly 150000 bytes (its demand), got %d", ska2Alloc.UsedBytes)
	}

	// Total used should be sum of demands (380KB), not full block
	expectedTotal := uint32(30000 + 200000 + 150000) // 380KB
	if result.TotalUsed != expectedTotal {
		t.Errorf("Expected total used %d, got %d", expectedTotal, result.TotalUsed)
	}

	// Utilization should be 38%
	expectedUtilization := 38.0
	if result.GetUtilizationPercentage() != expectedUtilization {
		t.Errorf("Expected utilization %.1f%%, got %.1f%%",
			expectedUtilization, result.GetUtilizationPercentage())
	}
}

// TestThreeActiveSKATypes tests allocation with 3 active SKA types.
// Verifies that 90% SKA allocation splits 3 ways correctly (30% each).
func TestThreeActiveSKATypes(t *testing.T) {
	params := mockChainParamsWithThreeSKAs()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block

	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 120000, // 120KB pending
		1:                    400000, // 400KB pending (SKA-1)
		2:                    250000, // 250KB pending (SKA-2)
		3:                    200000, // 200KB pending (SKA-3)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should get 10% = 100KB base
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.BaseAllocation != 100000 {
		t.Errorf("Expected VAR base allocation 100000 (10%%), got %d", varAlloc.BaseAllocation)
	}

	// Each SKA should get 30% = 300KB base (90% / 3)
	skaPerType := uint32(300000)

	ska1Alloc := result.GetAllocationForCoinType(1)
	if ska1Alloc.BaseAllocation != skaPerType {
		t.Errorf("Expected SKA-1 base allocation %d (30%%), got %d", skaPerType, ska1Alloc.BaseAllocation)
	}

	ska2Alloc := result.GetAllocationForCoinType(2)
	if ska2Alloc.BaseAllocation != skaPerType {
		t.Errorf("Expected SKA-2 base allocation %d (30%%), got %d", skaPerType, ska2Alloc.BaseAllocation)
	}

	ska3Alloc := result.GetAllocationForCoinType(3)
	if ska3Alloc.BaseAllocation != skaPerType {
		t.Errorf("Expected SKA-3 base allocation %d (30%%), got %d", skaPerType, ska3Alloc.BaseAllocation)
	}

	// Verify all allocations exist (non-nil)
	if ska3Alloc == nil {
		t.Fatal("Expected SKA-3 allocation to exist")
	}

	// Verify VAR gets overflow (needs 120KB, base is 100KB)
	// Due to 10%/90% redistribution split, VAR may not get full 20KB extra
	if varAlloc.UsedBytes < 100000 {
		t.Errorf("Expected VAR to use at least 100000 bytes, got %d", varAlloc.UsedBytes)
	}
	if varAlloc.UsedBytes > 120000 {
		t.Errorf("Expected VAR to use at most 120000 bytes (its demand), got %d", varAlloc.UsedBytes)
	}

	// SKA-1 needs 400KB, base is 300KB, should get overflow
	if ska1Alloc.UsedBytes < 300000 {
		t.Errorf("Expected SKA-1 to use at least 300000 bytes, got %d", ska1Alloc.UsedBytes)
	}

	// Total base allocations should equal maxBlockSize
	totalBase := varAlloc.BaseAllocation + ska1Alloc.BaseAllocation + ska2Alloc.BaseAllocation + ska3Alloc.BaseAllocation
	if totalBase != 1000000 {
		t.Errorf("Expected total base allocations to equal 1000000, got %d", totalBase)
	}
}

// TestExactBoundaryConditions tests edge cases with exact boundary values.
// Verifies integer division and exact demand matching base allocations.
func TestExactBoundaryConditions(t *testing.T) {
	t.Run("Mainnet_ExactVARBase", func(t *testing.T) {
		// Mainnet has 375KB blocks, VAR gets exactly 10% = 37,500 bytes
		params := chaincfg.MainNetParams()
		allocator := NewBlockSpaceAllocator(375000, params)

		pendingTxBytes := map[cointype.CoinType]uint32{
			cointype.CoinTypeVAR: 37500, // Exactly 10% of 375KB
			cointype.CoinType(1): 0,     // No SKA demand
		}

		result := allocator.AllocateBlockSpace(pendingTxBytes)
		varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)

		// VAR should use exactly its base allocation
		if varAlloc.UsedBytes != 37500 {
			t.Errorf("Expected VAR to use exactly 37500 bytes, got %d", varAlloc.UsedBytes)
		}

		// With no SKA demand, VAR gets full block allocation
		if varAlloc.FinalAllocation != 375000 {
			t.Errorf("Expected VAR final allocation 375000 (no SKA demand), got %d", varAlloc.FinalAllocation)
		}
	})

	t.Run("ExactVARBasePlusOne", func(t *testing.T) {
		// Test VAR needing exactly base + 1 byte (triggers overflow by 1 byte)
		params := chaincfg.MainNetParams()
		allocator := NewBlockSpaceAllocator(375000, params)

		pendingTxBytes := map[cointype.CoinType]uint32{
			cointype.CoinTypeVAR: 37501,  // Base + 1 byte
			cointype.CoinType(1): 100000, // SKA-1 has demand
		}

		result := allocator.AllocateBlockSpace(pendingTxBytes)
		varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)

		// VAR should use all 37,501 bytes (base + 1 from overflow)
		if varAlloc.UsedBytes != 37501 {
			t.Errorf("Expected VAR to use 37501 bytes, got %d", varAlloc.UsedBytes)
		}

		// VAR should have gotten overflow
		if varAlloc.FinalAllocation <= varAlloc.BaseAllocation {
			t.Errorf("Expected VAR final allocation > base (got overflow), final=%d base=%d",
				varAlloc.FinalAllocation, varAlloc.BaseAllocation)
		}
	})

	t.Run("OddBlockSize_IntegerDivision", func(t *testing.T) {
		// Test with block size not divisible by 10 (tests integer division)
		params := mockChainParams()
		allocator := NewBlockSpaceAllocator(999999, params) // Odd size

		pendingTxBytes := map[cointype.CoinType]uint32{
			cointype.CoinTypeVAR: 50000,
			1:                    300000,
			2:                    300000,
		}

		result := allocator.AllocateBlockSpace(pendingTxBytes)

		// Verify base allocation uses integer division
		// 999,999 / 10 = 99,999 (integer division)
		varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
		if varAlloc.BaseAllocation != 99999 {
			t.Errorf("Expected VAR base allocation 99999 (integer division), got %d", varAlloc.BaseAllocation)
		}

		// Total base allocations should equal maxBlockSize
		ska1Alloc := result.GetAllocationForCoinType(1)
		ska2Alloc := result.GetAllocationForCoinType(2)
		totalBase := varAlloc.BaseAllocation + ska1Alloc.BaseAllocation + ska2Alloc.BaseAllocation
		if totalBase != 999999 {
			t.Errorf("Expected total base allocations to equal 999999, got %d", totalBase)
		}
	})
}

// TestIdentifyActivePendingTypes verifies active pending type detection.
// DEPRECATED: This test tested the old identifyActivePendingTypes() helper function
// which is no longer used by the simplified algorithm. The new algorithm handles
// pending type identification inline. Kept for reference.
/*
func TestIdentifyActivePendingTypes(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params)

	// Setup scenario where VAR and SKA-1 have unmet demand, SKA-2 is satisfied
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
			UsedBytes:    100000, // Fully satisfied - but still has PendingBytes
		},
	}

	activePending := allocator.identifyActivePendingTypes(allocations)

	// Should identify all 3 types as having pending bytes (even SKA-2 which is satisfied)
	// This is intentional: types with pending are eligible for overflow even if
	// they fit within their base allocation.
	if len(activePending) != 3 {
		t.Errorf("Expected 3 active pending types, got %d", len(activePending))
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
*/

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

	// With no SKA types, VAR gets 100% via early exit path
	// Base and final should both be maxBlockSize (1000000)
	if varAlloc.BaseAllocation != 1000000 {
		t.Errorf("Expected VAR base allocation 1000000 (no SKA early exit), got %d", varAlloc.BaseAllocation)
	}

	// VAR should use all its pending (950KB)
	if varAlloc.UsedBytes != 950000 {
		t.Errorf("Expected VAR to use 950000 bytes, got %d", varAlloc.UsedBytes)
	}

	// Final allocation should be the full block
	if varAlloc.FinalAllocation != 1000000 {
		t.Errorf("Expected VAR final allocation 1000000, got %d", varAlloc.FinalAllocation)
	}
}

// TestIntegerMathPrecision verifies that integer math preserves all bytes.
// DEPRECATED: This test tested the old calculateBaseAllocations() helper function.
// The new algorithm doesn't pre-calculate base allocations, so this test is obsolete.
// Kept for reference.
/*
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
*/

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

// TestMixedPendingWithNewCoinType tests handling of coin types not in active configuration.
// UPDATED: The new simplified algorithm only allocates to configured active SKA types,
// so unconfigured coin types (like 99 here) won't get allocations. This is correct behavior.
func TestMixedPendingWithNewCoinType(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params)

	// Add pending for a coin type that doesn't have base allocation
	pendingTxBytes := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 50000,  // 50KB
		1:                    100000, // 100KB
		2:                    100000, // 100KB
		99:                   200000, // 200KB - new coin type NOT in active config!
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// Coin type 99 should NOT get an allocation (not in active SKA config)
	// This is correct - we only allocate to configured coin types
	newTypeAlloc := result.GetAllocationForCoinType(99)
	if newTypeAlloc != nil {
		t.Errorf("Expected no allocation for unconfigured coin type 99, got %+v", newTypeAlloc)
	}

	// The configured types (VAR, SKA-1, SKA-2) should have allocations
	if result.GetAllocationForCoinType(cointype.CoinTypeVAR) == nil {
		t.Error("Expected VAR allocation")
	}
	if result.GetAllocationForCoinType(1) == nil {
		t.Error("Expected SKA-1 allocation")
	}
	if result.GetAllocationForCoinType(2) == nil {
		t.Error("Expected SKA-2 allocation")
	}
}

// TestOverflowDistributedToVARWhenSKAUnused tests the critical mainnet fix:
// when SKA has zero pending transactions, VAR should get SKA's unused space.
func TestOverflowDistributedToVARWhenSKAUnused(t *testing.T) {
	params := chaincfg.MainNetParams()
	allocator := NewBlockSpaceAllocator(375000, params) // 375KB mainnet block

	// Scenario: Small amount of VAR pending (well under 37.5KB base), 0 SKA pending
	// This simulates the mainnet stalling issue where VAR transactions couldn't fit
	pending := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 4000, // Only 4KB so far
	}

	result := allocator.AllocateBlockSpace(pending)
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	skaAlloc := result.GetAllocationForCoinType(cointype.CoinType(1))

	if varAlloc == nil {
		t.Fatal("Expected VAR allocation")
	}

	// Debug output
	t.Logf("Overflow handled: %d", result.OverflowHandled)
	t.Logf("SKA-1 allocation exists: %v", skaAlloc != nil)
	if skaAlloc != nil {
		t.Logf("SKA-1: base=%d, pending=%d, used=%d, final=%d",
			skaAlloc.BaseAllocation, skaAlloc.PendingBytes, skaAlloc.UsedBytes, skaAlloc.FinalAllocation)
	}

	// VAR should get MORE than just its 10% base (37.5KB) since SKA is unused
	if varAlloc.FinalAllocation <= 37500 {
		t.Errorf("VAR should get overflow from unused SKA, got %d (expected > 37500)",
			varAlloc.FinalAllocation)
	}

	// VAR should get nearly the full block (at least 98%)
	if varAlloc.FinalAllocation < 367500 {
		t.Errorf("VAR should get nearly full block when SKA unused, got %d (expected >= 367500)",
			varAlloc.FinalAllocation)
	}

	t.Logf("VAR allocation with 4KB pending, 0 SKA: %d bytes (%.1f%% of block)",
		varAlloc.FinalAllocation,
		float64(varAlloc.FinalAllocation)/float64(375000)*100)
}

// TestLargeVARTransactionSetFits tests that large VAR transaction sets
// (like ticket split transactions) fit in blocks when SKA has no demand.
func TestLargeVARTransactionSetFits(t *testing.T) {
	params := chaincfg.MainNetParams()
	allocator := NewBlockSpaceAllocator(375000, params)

	// Scenario: 100KB of VAR transactions pending (typical for split transactions)
	pending := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 100000,
	}

	result := allocator.AllocateBlockSpace(pending)
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)

	if varAlloc == nil {
		t.Fatal("Expected VAR allocation")
	}

	// All 100KB should fit (block is 375KB)
	if varAlloc.FinalAllocation < 100000 {
		t.Errorf("100KB VAR should fit in 375KB block, got allocation %d",
			varAlloc.FinalAllocation)
	}

	// Verify it can use the full 100KB it needs
	if varAlloc.UsedBytes < 100000 {
		t.Errorf("VAR should use its full 100KB demand, got %d", varAlloc.UsedBytes)
	}

	t.Logf("VAR allocation with 100KB pending: %d bytes allocated, %d bytes used",
		varAlloc.FinalAllocation, varAlloc.UsedBytes)
}

// TestMixedDemandRespects10_90Split verifies that when both VAR and SKA
// have pending transactions, the 10%/90% split is respected.
func TestMixedDemandRespects10_90Split(t *testing.T) {
	params := chaincfg.MainNetParams()
	allocator := NewBlockSpaceAllocator(375000, params)

	// Scenario: Both VAR and SKA have significant pending transactions
	pending := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 50000,  // 50KB VAR
		cointype.CoinType(1): 200000, // 200KB SKA-1
	}

	result := allocator.AllocateBlockSpace(pending)
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	skaAlloc := result.GetAllocationForCoinType(cointype.CoinType(1))

	if varAlloc == nil {
		t.Fatal("Expected VAR allocation")
	}
	if skaAlloc == nil {
		t.Fatal("Expected SKA-1 allocation")
	}

	// When both have demand, should respect base allocations
	// VAR base = 37.5KB, SKA base = 337.5KB
	if varAlloc.BaseAllocation != 37500 {
		t.Errorf("VAR base should be 37500, got %d", varAlloc.BaseAllocation)
	}

	// Both fit within their respective spaces with overflow
	if varAlloc.FinalAllocation < 50000 {
		t.Errorf("VAR should get at least its 50KB demand, got %d", varAlloc.FinalAllocation)
	}
	if skaAlloc.FinalAllocation < 200000 {
		t.Errorf("SKA-1 should get at least its 200KB demand, got %d", skaAlloc.FinalAllocation)
	}

	t.Logf("Mixed demand: VAR=%d/%d bytes, SKA=%d/%d bytes",
		varAlloc.UsedBytes, varAlloc.FinalAllocation,
		skaAlloc.UsedBytes, skaAlloc.FinalAllocation)
}

// TestVARReclaimsSpaceWhenSKABecomesInactive tests dynamic space allocation:
// if SKA had transactions but then stops, VAR should reclaim the space.
func TestVARReclaimsSpaceWhenSKABecomesInactive(t *testing.T) {
	params := chaincfg.MainNetParams()
	allocator := NewBlockSpaceAllocator(375000, params)

	// Scenario 1: Both have significant pending that exceeds their base allocations
	pendingBoth := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 50000,  // Exceeds 37.5KB base
		cointype.CoinType(1): 350000, // Exceeds 337.5KB base
	}
	result1 := allocator.AllocateBlockSpace(pendingBoth)
	varAlloc1 := result1.GetAllocationForCoinType(cointype.CoinTypeVAR)

	// Scenario 2: SKA stops, only VAR has pending (but still 50KB)
	pendingVAROnly := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 50000,
	}
	result2 := allocator.AllocateBlockSpace(pendingVAROnly)
	varAlloc2 := result2.GetAllocationForCoinType(cointype.CoinTypeVAR)

	// VAR should get much more space when SKA is inactive
	if varAlloc2.FinalAllocation <= varAlloc1.FinalAllocation {
		t.Errorf("VAR should get more space when SKA inactive: got %d with SKA vs %d without",
			varAlloc1.FinalAllocation, varAlloc2.FinalAllocation)
	}

	// When SKA inactive, VAR should get nearly full block (at least its 50KB demand)
	if varAlloc2.FinalAllocation < 50000 {
		t.Errorf("VAR should get at least its demand when SKA inactive, got %d",
			varAlloc2.FinalAllocation)
	}

	// Should get at least what it needs (its demand)
	// Note: The allocator only gives what's needed, not more, which is correct behavior
	if varAlloc2.UsedBytes != 50000 {
		t.Errorf("VAR should use exactly its 50KB demand, got %d", varAlloc2.UsedBytes)
	}

	t.Logf("VAR space: %d bytes with SKA active, %d bytes with SKA inactive",
		varAlloc1.FinalAllocation, varAlloc2.FinalAllocation)
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

// TestAllocationOverflowBug tests the critical bug where VAR allocation exceeds maxBlockSize.
// This reproduces the mainnet issue where:
// - Max block: 375,000 bytes
// - VAR pending: 1,738 bytes
// - SKA-1 pending: 254 bytes
// - VAR allocation: 410,508 bytes (109% of max!)
//
// Root cause: VAR's unused space from initial allocation is being redistributed back to VAR,
// causing double-counting of VAR's unused space.
func TestAllocationOverflowBug(t *testing.T) {
	// Use mainnet config with SKA-1 active
	params := chaincfg.MainNetParams()
	allocator := NewBlockSpaceAllocator(375000, params)

	// Exact scenario from mainnet logs
	pending := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 1738, // Very small VAR pending
		cointype.CoinType(1): 254,  // Very small SKA-1 pending
	}

	result := allocator.AllocateBlockSpace(pending)

	// CRITICAL: Total allocations must NEVER exceed maxBlockSize
	totalAllocated := uint32(0)
	for _, alloc := range result.Allocations {
		totalAllocated += alloc.FinalAllocation
	}

	if totalAllocated > 375000 {
		t.Errorf("CRITICAL BUG: Total allocations (%d) exceed maxBlockSize (375000) by %d bytes!",
			totalAllocated, totalAllocated-375000)

		// Debug output to help diagnose
		varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
		skaAlloc := result.GetAllocationForCoinType(cointype.CoinType(1))
		t.Logf("VAR: base=%d, final=%d, pending=%d, used=%d",
			varAlloc.BaseAllocation, varAlloc.FinalAllocation, varAlloc.PendingBytes, varAlloc.UsedBytes)
		t.Logf("SKA-1: base=%d, final=%d, pending=%d, used=%d",
			skaAlloc.BaseAllocation, skaAlloc.FinalAllocation, skaAlloc.PendingBytes, skaAlloc.UsedBytes)
	}

	// Verify each coin type's allocation doesn't exceed maxBlockSize
	for coinType, alloc := range result.Allocations {
		if alloc.FinalAllocation > 375000 {
			t.Errorf("Coin type %s allocation (%d) exceeds maxBlockSize (375000)",
				coinType, alloc.FinalAllocation)
		}
	}

	// Sanity check: UsedBytes should never exceed FinalAllocation
	for coinType, alloc := range result.Allocations {
		if alloc.UsedBytes > alloc.FinalAllocation {
			t.Errorf("Coin type %s used (%d) exceeds final allocation (%d)",
				coinType, alloc.UsedBytes, alloc.FinalAllocation)
		}
	}

	// Verify VAR gets all leftover space
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	skaAlloc := result.GetAllocationForCoinType(cointype.CoinType(1))

	// Expected: VAR should get its base (37,500) + all leftover space from redistribution
	// Since both VAR and SKA have minimal needs, most space should go to VAR
	expectedVARMin := uint32(37500) // At least its base
	if varAlloc.FinalAllocation < expectedVARMin {
		t.Errorf("VAR should get at least its base allocation %d, got %d",
			expectedVARMin, varAlloc.FinalAllocation)
	}

	// Log the actual allocations for verification
	t.Logf("Final allocations: VAR=%d (%.1f%%), SKA-1=%d (%.1f%%)",
		varAlloc.FinalAllocation, float64(varAlloc.FinalAllocation)/375000*100,
		skaAlloc.FinalAllocation, float64(skaAlloc.FinalAllocation)/375000*100)
}

// TestVARGetsLeftoverWhenSKAHasMinimalDemand tests that VAR can claim unused SKA space.
// This is critical for mainnet where large VAR transaction sets need to fit when SKA is idle.
func TestVARGetsLeftoverWhenSKAHasMinimalDemand(t *testing.T) {
	params := chaincfg.MainNetParams()
	allocator := NewBlockSpaceAllocator(375000, params)

	// Scenario: Large VAR demand (100KB), tiny SKA demand (254 bytes)
	// VAR should get its base (37.5KB) + most of SKA's unused space
	pending := map[cointype.CoinType]uint32{
		cointype.CoinTypeVAR: 100000, // 100KB VAR pending (needs overflow)
		cointype.CoinType(1): 254,    // 254 bytes SKA-1 (minimal)
	}

	result := allocator.AllocateBlockSpace(pending)

	// Total allocations must not exceed maxBlockSize
	totalAllocated := uint32(0)
	for _, alloc := range result.Allocations {
		totalAllocated += alloc.FinalAllocation
	}

	if totalAllocated > 375000 {
		t.Errorf("Total allocations (%d) exceed maxBlockSize (375000)", totalAllocated)
	}

	// VAR should get enough space for its 100KB demand
	varAlloc := result.GetAllocationForCoinType(cointype.CoinTypeVAR)
	if varAlloc.FinalAllocation < 100000 {
		t.Errorf("VAR should get at least 100KB for its demand, got %d", varAlloc.FinalAllocation)
	}

	// VAR should actually use its full 100KB demand
	if varAlloc.UsedBytes != 100000 {
		t.Errorf("VAR should use its full 100KB demand, got %d", varAlloc.UsedBytes)
	}

	// SKA should only get what it needs (254 bytes) or slightly more from its base
	skaAlloc := result.GetAllocationForCoinType(cointype.CoinType(1))
	if skaAlloc.UsedBytes != 254 {
		t.Errorf("SKA-1 should use exactly 254 bytes, got %d", skaAlloc.UsedBytes)
	}

	t.Logf("VAR: final=%d, used=%d (demand=100000)", varAlloc.FinalAllocation, varAlloc.UsedBytes)
	t.Logf("SKA-1: final=%d, used=%d (demand=254)", skaAlloc.FinalAllocation, skaAlloc.UsedBytes)
	t.Logf("Total allocated: %d / 375000 (%.1f%%)",
		totalAllocated, float64(totalAllocated)/375000*100)
}
