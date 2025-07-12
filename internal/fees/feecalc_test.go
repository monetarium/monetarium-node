// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package fees

import (
	"testing"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
)

// TestNewCoinTypeFeeCalculator tests the creation of a new fee calculator
func TestNewCoinTypeFeeCalculator(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4) // 10000 atoms/KB
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	if calc == nil {
		t.Fatal("Expected fee calculator to be created")
	}
	
	if calc.chainParams != params {
		t.Error("Chain params not set correctly")
	}
	
	if calc.defaultMinRelayFee != defaultMinRelayFee {
		t.Errorf("Expected default min relay fee %d, got %d", defaultMinRelayFee, calc.defaultMinRelayFee)
	}
	
	// Check that VAR and SKA fee rates are initialized
	supportedTypes := calc.GetSupportedCoinTypes()
	if len(supportedTypes) != 2 {
		t.Errorf("Expected 2 supported coin types, got %d", len(supportedTypes))
	}
	
	varFound, skaFound := false, false
	for _, coinType := range supportedTypes {
		if coinType == wire.CoinTypeVAR {
			varFound = true
		} else if coinType == wire.CoinTypeSKA {
			skaFound = true
		}
	}
	
	if !varFound {
		t.Error("VAR coin type not found in supported types")
	}
	if !skaFound {
		t.Error("SKA coin type not found in supported types")
	}
}

// TestCalculateMinFee tests minimum fee calculation for different coin types
func TestCalculateMinFee(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4) // 10000 atoms/KB
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	tests := []struct {
		name         string
		serializedSize int64
		coinType     wire.CoinType
		expectedMin  int64
	}{
		{
			name:         "VAR transaction 250 bytes",
			serializedSize: 250,
			coinType:     wire.CoinTypeVAR,
			expectedMin:  2500, // (250 * 10000) / 1000 = 2500 atoms
		},
		{
			name:         "SKA transaction 250 bytes",
			serializedSize: 250,
			coinType:     wire.CoinTypeSKA,
			expectedMin:  250, // SKA has 10x lower fee: (250 * 1000) / 1000 = 250 atoms
		},
		{
			name:         "Large VAR transaction 1000 bytes",
			serializedSize: 1000,
			coinType:     wire.CoinTypeVAR,
			expectedMin:  10000, // (1000 * 10000) / 1000 = 10000 atoms
		},
		{
			name:         "Large SKA transaction 1000 bytes",
			serializedSize: 1000,
			coinType:     wire.CoinTypeSKA,
			expectedMin:  1000, // (1000 * 1000) / 1000 = 1000 atoms
		},
		{
			name:         "Unknown coin type defaults to VAR",
			serializedSize: 500,
			coinType:     wire.CoinType(99),
			expectedMin:  5000, // Should default to VAR: (500 * 10000) / 1000 = 5000 atoms
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			minFee := calc.CalculateMinFee(test.serializedSize, test.coinType)
			if minFee != test.expectedMin {
				t.Errorf("Expected minimum fee %d, got %d", test.expectedMin, minFee)
			}
		})
	}
}

// TestEstimateFeeRate tests fee rate estimation for different confirmation targets
func TestEstimateFeeRate(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	tests := []struct {
		name                string
		coinType           wire.CoinType
		targetConfirmations int
		expectError        bool
	}{
		{
			name:                "VAR next block",
			coinType:           wire.CoinTypeVAR,
			targetConfirmations: 1,
			expectError:        false,
		},
		{
			name:                "SKA fast confirmation",
			coinType:           wire.CoinTypeSKA,
			targetConfirmations: 3,
			expectError:        false,
		},
		{
			name:                "VAR normal confirmation",
			coinType:           wire.CoinTypeVAR,
			targetConfirmations: 6,
			expectError:        false,
		},
		{
			name:                "Unknown coin type",
			coinType:           wire.CoinType(99),
			targetConfirmations: 1,
			expectError:        true,
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feeRate, err := calc.EstimateFeeRate(test.coinType, test.targetConfirmations)
			
			if test.expectError {
				if err == nil {
					t.Error("Expected error for unknown coin type")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if feeRate <= 0 {
				t.Error("Expected positive fee rate")
			}
			
			// Next block should have higher fee than normal confirmation
			if test.targetConfirmations == 1 {
				normalRate, _ := calc.EstimateFeeRate(test.coinType, 6)
				if feeRate <= normalRate {
					t.Error("Next block fee should be higher than normal confirmation fee")
				}
			}
		})
	}
}

// TestUpdateUtilization tests network utilization updates and dynamic fee adjustment
func TestUpdateUtilization(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	// Get initial fee rate
	initialStats, err := calc.GetFeeStats(wire.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get initial fee stats: %v", err)
	}
	initialMultiplier := initialStats.DynamicFeeMultiplier
	
	// Update with high utilization
	calc.UpdateUtilization(wire.CoinTypeVAR, 150, 50000, 0.95) // 95% block space used, 150 pending txs
	
	// Get updated fee rate
	updatedStats, err := calc.GetFeeStats(wire.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get updated fee stats: %v", err)
	}
	
	if updatedStats.DynamicFeeMultiplier <= initialMultiplier {
		t.Error("Expected fee multiplier to increase with high utilization")
	}
	
	if updatedStats.PendingTxCount != 150 {
		t.Errorf("Expected pending tx count 150, got %d", updatedStats.PendingTxCount)
	}
	
	if updatedStats.PendingTxSize != 50000 {
		t.Errorf("Expected pending tx size 50000, got %d", updatedStats.PendingTxSize)
	}
	
	if updatedStats.BlockSpaceUsed != 0.95 {
		t.Errorf("Expected block space used 0.95, got %f", updatedStats.BlockSpaceUsed)
	}
}

// TestRecordTransactionFee tests transaction fee recording for fee estimation
func TestRecordTransactionFee(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	// Record some transaction fees
	calc.RecordTransactionFee(wire.CoinTypeVAR, 5000, 250, true)  // 20000 atoms/KB
	calc.RecordTransactionFee(wire.CoinTypeVAR, 7500, 250, true)  // 30000 atoms/KB
	calc.RecordTransactionFee(wire.CoinTypeVAR, 10000, 250, true) // 40000 atoms/KB
	
	// Get fee stats
	stats, err := calc.GetFeeStats(wire.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get fee stats: %v", err)
	}
	
	// Check that fee rates are calculated
	if stats.FastFee == 0 {
		t.Error("Expected non-zero fast fee")
	}
	if stats.NormalFee == 0 {
		t.Error("Expected non-zero normal fee")
	}
	if stats.SlowFee == 0 {
		t.Error("Expected non-zero slow fee")
	}
	
	// Fast fee should be >= normal fee >= slow fee
	if stats.FastFee < stats.NormalFee {
		t.Error("Fast fee should be >= normal fee")
	}
	if stats.NormalFee < stats.SlowFee {
		t.Error("Normal fee should be >= slow fee")
	}
}

// TestValidateTransactionFees tests transaction fee validation
func TestValidateTransactionFees(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	tests := []struct {
		name          string
		txFee         int64
		serializedSize int64
		coinType      wire.CoinType
		allowHighFees bool
		expectError   bool
		errorContains string
	}{
		{
			name:          "VAR sufficient fee",
			txFee:         3000,
			serializedSize: 250,
			coinType:      wire.CoinTypeVAR,
			allowHighFees: false,
			expectError:   false,
		},
		{
			name:          "VAR insufficient fee",
			txFee:         1000,
			serializedSize: 250,
			coinType:      wire.CoinTypeVAR,
			allowHighFees: false,
			expectError:   true,
			errorContains: "insufficient fee",
		},
		{
			name:          "SKA sufficient fee",
			txFee:         300,
			serializedSize: 250,
			coinType:      wire.CoinTypeSKA,
			allowHighFees: false,
			expectError:   false,
		},
		{
			name:          "SKA insufficient fee",
			txFee:         100,
			serializedSize: 250,
			coinType:      wire.CoinTypeSKA,
			allowHighFees: false,
			expectError:   true,
			errorContains: "insufficient fee",
		},
		{
			name:          "VAR excessive fee allowed",
			txFee:         1000000, // Very high fee
			serializedSize: 250,
			coinType:      wire.CoinTypeVAR,
			allowHighFees: true,
			expectError:   false,
		},
		{
			name:          "VAR excessive fee rejected",
			txFee:         1000000, // Very high fee
			serializedSize: 250,
			coinType:      wire.CoinTypeVAR,
			allowHighFees: false,
			expectError:   true,
			errorContains: "fee too high",
		},
	}
	
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := calc.ValidateTransactionFees(test.txFee, test.serializedSize, 
				test.coinType, test.allowHighFees)
			
			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
					return
				}
				if test.errorContains != "" && !containsString(err.Error(), test.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %s", test.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestDynamicFeeAdjustment tests that fees adjust properly based on network conditions
func TestDynamicFeeAdjustment(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	// Get baseline fee
	baselineFee := calc.CalculateMinFee(250, wire.CoinTypeVAR)
	
	// Simulate high network utilization
	calc.UpdateUtilization(wire.CoinTypeVAR, 200, 100000, 0.9) // High utilization
	
	// Fee should increase
	highUtilFee := calc.CalculateMinFee(250, wire.CoinTypeVAR)
	if highUtilFee <= baselineFee {
		t.Error("Expected fee to increase with high utilization")
	}
	
	// Simulate low network utilization
	calc.UpdateUtilization(wire.CoinTypeVAR, 5, 1000, 0.1) // Low utilization
	
	// Allow time for smoothing
	time.Sleep(time.Millisecond * 10)
	calc.UpdateUtilization(wire.CoinTypeVAR, 5, 1000, 0.1) // Update again
	
	lowUtilFee := calc.CalculateMinFee(250, wire.CoinTypeVAR)
	if lowUtilFee >= highUtilFee {
		t.Error("Expected fee to decrease with low utilization")
	}
}

// TestFeeStatsConsistency tests that fee stats are consistent
func TestFeeStatsConsistency(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	// Record various fees
	fees := []int64{1000, 2000, 3000, 4000, 5000}
	for _, fee := range fees {
		calc.RecordTransactionFee(wire.CoinTypeVAR, fee, 250, true)
	}
	
	stats, err := calc.GetFeeStats(wire.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get fee stats: %v", err)
	}
	
	// Check basic consistency
	if stats.DynamicFeeMultiplier < 0.1 || stats.DynamicFeeMultiplier > 20.0 {
		t.Errorf("Dynamic fee multiplier %f is out of reasonable range", stats.DynamicFeeMultiplier)
	}
	
	if stats.MinRelayFee <= 0 {
		t.Error("Min relay fee should be positive")
	}
	
	if stats.MaxFeeRate <= stats.MinRelayFee {
		t.Error("Max fee rate should be greater than min relay fee")
	}
	
	if stats.LastUpdated.IsZero() {
		t.Error("Last updated time should be set")
	}
}

// TestSKASpecificFeeBehavior tests SKA-specific fee behavior
func TestSKASpecificFeeBehavior(t *testing.T) {
	params := chaincfg.SimNetParams()
	params.SKAMinRelayTxFee = 500 // Set custom SKA fee rate
	defaultMinRelayFee := dcrutil.Amount(1e4)
	
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)
	
	// SKA should use custom fee rate
	skaFee := calc.CalculateMinFee(1000, wire.CoinTypeSKA) // 1KB transaction
	expectedSKAFee := int64(500) // Should use params.SKAMinRelayTxFee
	
	if skaFee != expectedSKAFee {
		t.Errorf("Expected SKA fee %d, got %d", expectedSKAFee, skaFee)
	}
	
	// VAR should still use default
	varFee := calc.CalculateMinFee(1000, wire.CoinTypeVAR)
	expectedVARFee := int64(10000) // Should use defaultMinRelayFee
	
	if varFee != expectedVARFee {
		t.Errorf("Expected VAR fee %d, got %d", expectedVARFee, varFee)
	}
	
	// SKA fees should be lower than VAR fees for same transaction size
	if skaFee >= varFee {
		t.Error("SKA fees should be lower than VAR fees")
	}
}

// containsString checks if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (s == substr || 
		    (len(substr) > 0 && 
		     (s[:len(substr)] == substr || 
		      s[len(s)-len(substr):] == substr || 
		      findSubstring(s, substr))))
}

// findSubstring helper function to find substring
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}