// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"math/big"
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
)

// TestFeesByType tests the basic functionality of FeesByType.
func TestFeesByType(t *testing.T) {
	// Test NewFeesByType
	fees := NewFeesByType()
	if fees == nil {
		t.Fatal("NewFeesByType returned nil")
	}
	if len(fees) != 0 {
		t.Errorf("Expected empty fees map, got %d entries", len(fees))
	}

	// Test Add and Get for VAR
	fees.Add(cointype.CoinTypeVAR, 1000)
	if got := fees.Get(cointype.CoinTypeVAR); got != 1000 {
		t.Errorf("Expected VAR fees 1000, got %d", got)
	}

	// Test Add and Get for SKA (uses int64 for backward compatibility)
	fees.Add(cointype.CoinType(1), 500)
	fees.Add(cointype.CoinType(2), 300)

	if got := fees.Get(cointype.CoinType(1)); got != 500 {
		t.Errorf("Expected SKA-1 fees 500, got %d", got)
	}
	if got := fees.Get(cointype.CoinType(2)); got != 300 {
		t.Errorf("Expected SKA-2 fees 300, got %d", got)
	}

	// Test Add to existing VAR
	fees.Add(cointype.CoinTypeVAR, 200)
	if got := fees.Get(cointype.CoinTypeVAR); got != 1200 {
		t.Errorf("Expected VAR fees 1200 after adding 200, got %d", got)
	}

	// Test Get for non-existent coin type
	if got := fees.Get(cointype.CoinType(99)); got != 0 {
		t.Errorf("Expected 0 for non-existent coin type, got %d", got)
	}
}

// TestFeesByTypeSKA tests SKA-specific bigint functionality.
func TestFeesByTypeSKA(t *testing.T) {
	fees := NewFeesByType()

	// Test AddSKA with bigint
	bigFee := new(big.Int).SetUint64(1e18) // 1 SKA coin worth of atoms
	fees.AddSKA(cointype.CoinType(1), bigFee)

	// Test GetSKA
	got := fees.GetSKA(cointype.CoinType(1))
	if got == nil {
		t.Fatal("Expected non-nil SKA fee")
	}
	if got.Cmp(bigFee) != 0 {
		t.Errorf("Expected SKA fee %v, got %v", bigFee, got)
	}

	// Test Get returns 0 for large SKA fees (doesn't fit int64)
	// Note: 1e18 fits in int64 (max ~9.2e18), but let's test the boundary
	if got := fees.Get(cointype.CoinType(1)); got != 1e18 {
		t.Errorf("Expected Get to return int64 value %d, got %d", int64(1e18), got)
	}

	// Test AddSKA accumulates
	fees.AddSKA(cointype.CoinType(1), bigFee)
	got = fees.GetSKA(cointype.CoinType(1))
	expected := new(big.Int).SetUint64(2e18)
	if got.Cmp(expected) != 0 {
		t.Errorf("Expected accumulated SKA fee %v, got %v", expected, got)
	}

	// Test GetSKA for non-existent coin type
	if got := fees.GetSKA(cointype.CoinType(99)); got != nil {
		t.Errorf("Expected nil for non-existent SKA type, got %v", got)
	}
}

// TestFeesByTypeTypes tests the Types method.
func TestFeesByTypeTypes(t *testing.T) {
	fees := NewFeesByType()

	// Test empty types
	types := fees.Types()
	if len(types) != 0 {
		t.Errorf("Expected no types for empty fees, got %d", len(types))
	}

	// Add fees and test types
	fees.Add(cointype.CoinTypeVAR, 1000)
	fees.Add(cointype.CoinType(1), 500)
	fees.AddSKA(cointype.CoinType(3), big.NewInt(300))

	types = fees.Types()
	expectedCount := 3 // VAR, SKA-1, SKA-3
	if len(types) != expectedCount {
		t.Errorf("Expected %d types, got %d", expectedCount, len(types))
	}

	// Check that all expected types are present
	typeSet := make(map[cointype.CoinType]bool)
	for _, ct := range types {
		typeSet[ct] = true
	}

	expectedTypes := []cointype.CoinType{cointype.CoinTypeVAR, cointype.CoinType(1), cointype.CoinType(3)}
	for _, expected := range expectedTypes {
		if !typeSet[expected] {
			t.Errorf("Expected coin type %d in types, but not found", expected)
		}
	}
}

// TestFeesByTypeSKATypes tests the SKATypes method.
func TestFeesByTypeSKATypes(t *testing.T) {
	fees := NewFeesByType()

	// Add fees
	fees.Add(cointype.CoinTypeVAR, 1000)
	fees.Add(cointype.CoinType(1), 500)
	fees.AddSKA(cointype.CoinType(2), big.NewInt(300))

	types := fees.SKATypes()
	expectedCount := 2 // SKA-1, SKA-2 (not VAR)
	if len(types) != expectedCount {
		t.Errorf("Expected %d SKA types, got %d", expectedCount, len(types))
	}
}

// TestFeesByTypeMerge tests the Merge method.
func TestFeesByTypeMerge(t *testing.T) {
	fees1 := NewFeesByType()
	fees1.Add(cointype.CoinTypeVAR, 1000)
	fees1.Add(cointype.CoinType(1), 500)

	fees2 := NewFeesByType()
	fees2.Add(cointype.CoinTypeVAR, 200)                // Should add to existing
	fees2.AddSKA(cointype.CoinType(2), big.NewInt(300)) // New SKA type

	fees1.Merge(fees2)

	// Check merged results
	if got := fees1.Get(cointype.CoinTypeVAR); got != 1200 {
		t.Errorf("Expected merged VAR fees 1200, got %d", got)
	}
	if got := fees1.Get(cointype.CoinType(1)); got != 500 {
		t.Errorf("Expected SKA-1 fees unchanged at 500, got %d", got)
	}
	if got := fees1.GetSKA(cointype.CoinType(2)); got == nil || got.Cmp(big.NewInt(300)) != 0 {
		t.Errorf("Expected new SKA-2 fees 300, got %v", got)
	}

	// Original fees2 should be unchanged
	if got := fees2.Get(cointype.CoinTypeVAR); got != 200 {
		t.Errorf("Expected fees2 VAR unchanged at 200, got %d", got)
	}
}

// TestFeesByTypeHasFee tests the HasFee method.
func TestFeesByTypeHasFee(t *testing.T) {
	fees := NewFeesByType()

	// Empty - no fees
	if fees.HasFee(cointype.CoinTypeVAR) {
		t.Error("Expected HasFee to return false for empty VAR")
	}
	if fees.HasFee(cointype.CoinType(1)) {
		t.Error("Expected HasFee to return false for empty SKA")
	}

	// Add fees
	fees.Add(cointype.CoinTypeVAR, 1000)
	fees.AddSKA(cointype.CoinType(1), big.NewInt(500))

	if !fees.HasFee(cointype.CoinTypeVAR) {
		t.Error("Expected HasFee to return true for VAR with fees")
	}
	if !fees.HasFee(cointype.CoinType(1)) {
		t.Error("Expected HasFee to return true for SKA with fees")
	}
	if fees.HasFee(cointype.CoinType(2)) {
		t.Error("Expected HasFee to return false for SKA without fees")
	}
}

// TestFeesByTypeHasSKAFees tests the HasSKAFees method.
func TestFeesByTypeHasSKAFees(t *testing.T) {
	fees := NewFeesByType()

	// Empty - no SKA fees
	if fees.HasSKAFees() {
		t.Error("Expected HasSKAFees to return false for empty fees")
	}

	// VAR only - no SKA fees
	fees.Add(cointype.CoinTypeVAR, 1000)
	if fees.HasSKAFees() {
		t.Error("Expected HasSKAFees to return false for VAR-only fees")
	}

	// Add SKA fees
	fees.AddSKA(cointype.CoinType(1), big.NewInt(500))
	if !fees.HasSKAFees() {
		t.Error("Expected HasSKAFees to return true after adding SKA fees")
	}
}

// TestGetPrimaryCoinType tests the GetPrimaryCoinType function.
func TestGetPrimaryCoinType(t *testing.T) {
	tests := []struct {
		name     string
		outputs  []*TxOut
		expected cointype.CoinType
	}{
		{
			name:     "empty transaction",
			outputs:  []*TxOut{},
			expected: cointype.CoinTypeVAR,
		},
		{
			name: "VAR only transaction",
			outputs: []*TxOut{
				{Value: 1000, CoinType: cointype.CoinTypeVAR},
				{Value: 500, CoinType: cointype.CoinTypeVAR},
			},
			expected: cointype.CoinTypeVAR,
		},
		{
			name: "SKA transaction",
			outputs: []*TxOut{
				{Value: 1000, CoinType: cointype.CoinType(1)},
				{Value: 500, CoinType: cointype.CoinType(1)},
			},
			expected: cointype.CoinType(1),
		},
		{
			name: "mixed transaction - first non-VAR wins",
			outputs: []*TxOut{
				{Value: 1000, CoinType: cointype.CoinTypeVAR},
				{Value: 500, CoinType: cointype.CoinType(2)},
				{Value: 300, CoinType: cointype.CoinType(1)},
			},
			expected: cointype.CoinType(2),
		},
		{
			name: "SKA-3 transaction",
			outputs: []*TxOut{
				{Value: 1000, CoinType: cointype.CoinType(3)},
			},
			expected: cointype.CoinType(3),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &MsgTx{
				TxOut: test.outputs,
			}

			result := GetPrimaryCoinType(tx)
			if result != test.expected {
				t.Errorf("Expected coin type %d, got %d", test.expected, result)
			}
		})
	}
}

// TestFeesByTypeEdgeCases tests edge cases and error conditions.
func TestFeesByTypeEdgeCases(t *testing.T) {
	fees := NewFeesByType()

	// Test adding negative fees (should still work for VAR)
	fees.Add(cointype.CoinTypeVAR, -100)
	if got := fees.Get(cointype.CoinTypeVAR); got != -100 {
		t.Errorf("Expected negative fees -100, got %d", got)
	}

	// Test large coin type values
	largeCoinType := cointype.CoinType(255) // Maximum coin type
	fees.AddSKA(largeCoinType, big.NewInt(1000))
	if got := fees.GetSKA(largeCoinType); got == nil || got.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("Expected fees for large coin type, got %v", got)
	}

	// Test Types() with negative VAR values
	types := fees.Types()
	expectedCount := 1 // Only the large coin type has positive fees (VAR is negative)
	if len(types) != expectedCount {
		t.Errorf("Expected %d types with positive fees, got %d", expectedCount, len(types))
	}
}

// TestCalcFeeSplitByCoinType tests the fee split calculation.
func TestCalcFeeSplitByCoinType(t *testing.T) {
	fees := NewFeesByType()
	fees.Add(cointype.CoinTypeVAR, 1000)
	fees.AddSKA(cointype.CoinType(1), big.NewInt(2000))

	// 50/50 split
	minerFees, stakerFees := CalcFeeSplitByCoinType(fees, 50, 50)

	// Check VAR split
	if got := minerFees.Get(cointype.CoinTypeVAR); got != 500 {
		t.Errorf("Expected miner VAR fees 500, got %d", got)
	}
	if got := stakerFees.Get(cointype.CoinTypeVAR); got != 500 {
		t.Errorf("Expected staker VAR fees 500, got %d", got)
	}

	// Check SKA split
	minerSKA := minerFees.GetSKA(cointype.CoinType(1))
	if minerSKA == nil || minerSKA.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("Expected miner SKA fees 1000, got %v", minerSKA)
	}
	stakerSKA := stakerFees.GetSKA(cointype.CoinType(1))
	if stakerSKA == nil || stakerSKA.Cmp(big.NewInt(1000)) != 0 {
		t.Errorf("Expected staker SKA fees 1000, got %v", stakerSKA)
	}
}

// TestCalcFeeSplitByCoinTypeRemainder tests remainder handling in fee split.
func TestCalcFeeSplitByCoinTypeRemainder(t *testing.T) {
	fees := NewFeesByType()
	fees.Add(cointype.CoinTypeVAR, 100) // 100 / 3 = 33, 33, remainder 34

	// 1/2 split (miner gets 1/3, staker gets 2/3)
	minerFees, stakerFees := CalcFeeSplitByCoinType(fees, 1, 2)

	// Miner gets 1/3 + remainder = 33 + 1 = 34
	if got := minerFees.Get(cointype.CoinTypeVAR); got != 34 {
		t.Errorf("Expected miner VAR fees 34 (with remainder), got %d", got)
	}
	// Staker gets 2/3 = 66
	if got := stakerFees.Get(cointype.CoinTypeVAR); got != 66 {
		t.Errorf("Expected staker VAR fees 66, got %d", got)
	}
}

// TestCalcFeeSplitNil tests fee split with nil input.
func TestCalcFeeSplitNil(t *testing.T) {
	minerFees, stakerFees := CalcFeeSplitByCoinType(nil, 50, 50)

	if len(minerFees) != 0 {
		t.Error("Expected empty miner fees for nil input")
	}
	if len(stakerFees) != 0 {
		t.Error("Expected empty staker fees for nil input")
	}
}
