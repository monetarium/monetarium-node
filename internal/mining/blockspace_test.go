// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"testing"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
)

// mockChainParams creates a test chain parameters with configured SKA types.
func mockChainParams() *chaincfg.Params {
	params := &chaincfg.Params{}

	// Configure 3 SKA types for testing
	params.SKACoins = map[dcrutil.CoinType]*chaincfg.SKACoinConfig{
		1: {
			CoinType: 1,
			Name:     "Skarb-1",
			Symbol:   "SKA-1",
			Active:   true,
		},
		2: {
			CoinType: 2,
			Name:     "Skarb-2",
			Symbol:   "SKA-2",
			Active:   true,
		},
		3: {
			CoinType: 3,
			Name:     "Skarb-3",
			Symbol:   "SKA-3",
			Active:   false, // Inactive for testing
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
	varAllocation := baseAllocations[dcrutil.CoinTypeVAR]
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

	pendingTxBytes := map[dcrutil.CoinType]uint32{
		dcrutil.CoinTypeVAR: 800000,  // 800KB pending
		1:                   1000000, // 1000KB pending (SKA-1)
		2:                   100000,  // 100KB pending (SKA-2)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// Verify VAR allocation: 100KB base + 35KB overflow = 135KB
	varAlloc := result.GetAllocationForCoinType(dcrutil.CoinTypeVAR)
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

	pendingTxBytes := map[dcrutil.CoinType]uint32{
		dcrutil.CoinTypeVAR: 0,      // No VAR demand
		1:                   500000, // 500KB pending (SKA-1)
		2:                   300000, // 300KB pending (SKA-2)
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should use 0 bytes
	varAlloc := result.GetAllocationForCoinType(dcrutil.CoinTypeVAR)
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

	pendingTxBytes := map[dcrutil.CoinType]uint32{
		dcrutil.CoinTypeVAR: 800000, // 800KB pending
		1:                   0,      // No SKA-1 demand
		2:                   0,      // No SKA-2 demand
	}

	result := allocator.AllocateBlockSpace(pendingTxBytes)

	// VAR should get 100% of overflow since it's the only one with pending
	varAlloc := result.GetAllocationForCoinType(dcrutil.CoinTypeVAR)
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

	pendingTxBytes := map[dcrutil.CoinType]uint32{
		dcrutil.CoinTypeVAR: 200000, // 200KB pending
		1:                   800000, // 800KB pending (SKA-1)
		2:                   700000, // 700KB pending (SKA-2)
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

	varAlloc := result.GetAllocationForCoinType(dcrutil.CoinTypeVAR)
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
	allocations := map[dcrutil.CoinType]*CoinTypeAllocation{
		dcrutil.CoinTypeVAR: {
			CoinType:     dcrutil.CoinTypeVAR,
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

	pendingTxBytes := map[dcrutil.CoinType]uint32{
		dcrutil.CoinTypeVAR: 800000,
		1:                   1000000,
		2:                   100000,
	}

	activePending := allocator.identifyActivePendingTypes(pendingTxBytes, allocations)

	// Should identify VAR and SKA-1 as having pending
	if len(activePending) != 2 {
		t.Errorf("Expected 2 active pending types, got %d", len(activePending))
	}

	// Verify specific types are included
	hasVAR := false
	hasSKA1 := false
	for _, coinType := range activePending {
		if coinType == dcrutil.CoinTypeVAR {
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

	pendingTxBytes := map[dcrutil.CoinType]uint32{
		dcrutil.CoinTypeVAR: 0, // No pending
		1:                   0, // No pending
		2:                   0, // No pending
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

// createMockTransaction creates a mock transaction with the specified coin type outputs.
func createMockTransaction(coinTypes []wire.CoinType) *dcrutil.Tx {
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

// TestGetTransactionCoinType verifies transaction coin type determination.
func TestGetTransactionCoinType(t *testing.T) {
	testCases := []struct {
		name         string
		coinTypes    []wire.CoinType
		expectedType dcrutil.CoinType
	}{
		{
			name:         "VAR only transaction",
			coinTypes:    []wire.CoinType{wire.CoinTypeVAR, wire.CoinTypeVAR},
			expectedType: dcrutil.CoinTypeVAR,
		},
		{
			name:         "SKA-1 only transaction",
			coinTypes:    []wire.CoinType{wire.CoinType(1), wire.CoinType(1)},
			expectedType: dcrutil.CoinType(1),
		},
		{
			name:         "Mixed transaction - VAR majority",
			coinTypes:    []wire.CoinType{wire.CoinTypeVAR, wire.CoinTypeVAR, wire.CoinType(1)},
			expectedType: dcrutil.CoinTypeVAR,
		},
		{
			name:         "Mixed transaction - SKA majority",
			coinTypes:    []wire.CoinType{wire.CoinTypeVAR, wire.CoinType(1), wire.CoinType(1)},
			expectedType: dcrutil.CoinType(1),
		},
		{
			name:         "Single output transaction",
			coinTypes:    []wire.CoinType{wire.CoinType(2)},
			expectedType: dcrutil.CoinType(2),
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

// TestTransactionSizeTracker verifies transaction size tracking functionality.
func TestTransactionSizeTracker(t *testing.T) {
	params := mockChainParams()
	allocator := NewBlockSpaceAllocator(1000000, params) // 1MB block
	tracker := NewTransactionSizeTracker(allocator)

	// Create test transactions
	varTx := createMockTransaction([]wire.CoinType{wire.CoinTypeVAR, wire.CoinTypeVAR})
	ska1Tx := createMockTransaction([]wire.CoinType{wire.CoinType(1), wire.CoinType(1)})
	ska2Tx := createMockTransaction([]wire.CoinType{wire.CoinType(2)})

	// Add transactions to tracker
	tracker.AddTransaction(varTx)
	tracker.AddTransaction(ska1Tx)
	tracker.AddTransaction(ska2Tx)

	// Verify sizes are tracked correctly
	varSize := tracker.GetSizeForCoinType(dcrutil.CoinTypeVAR)
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
	varTx := createMockTransaction([]wire.CoinType{wire.CoinTypeVAR})

	// First transaction should be addable
	if !tracker.CanAddTransaction(varTx) {
		t.Error("First VAR transaction should be addable")
	}

	// Add the transaction
	tracker.AddTransaction(varTx)

	// Create a very large transaction that would exceed allocation
	largeCoinTypes := make([]wire.CoinType, 100) // Large transaction
	for i := range largeCoinTypes {
		largeCoinTypes[i] = wire.CoinTypeVAR
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
	varTx := createMockTransaction([]wire.CoinType{wire.CoinTypeVAR})
	tracker.AddTransaction(varTx)

	// Verify transaction is tracked
	if tracker.GetSizeForCoinType(dcrutil.CoinTypeVAR) == 0 {
		t.Error("Expected VAR size to be tracked before reset")
	}

	// Reset tracker
	tracker.Reset()

	// Verify all sizes are cleared
	if tracker.GetSizeForCoinType(dcrutil.CoinTypeVAR) != 0 {
		t.Error("Expected VAR size to be 0 after reset")
	}
}
