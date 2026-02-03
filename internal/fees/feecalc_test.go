// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package fees

import (
	"math/big"
	"testing"
	"time"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
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

	// Check that VAR and active SKA fee rates are initialized
	supportedTypes := calc.GetSupportedCoinTypes()

	// Count expected types: VAR + active SKA coins
	expectedCount := 1 // VAR
	for _, config := range params.SKACoins {
		if config.Active {
			expectedCount++
		}
	}

	if len(supportedTypes) != expectedCount {
		t.Errorf("Expected %d supported coin types, got %d", expectedCount, len(supportedTypes))
	}

	varFound, skaFound := false, false
	for _, coinType := range supportedTypes {
		if coinType == cointype.CoinTypeVAR {
			varFound = true
		} else if coinType == cointype.CoinType(1) { // SKA-1
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
		name           string
		serializedSize int64
		coinType       cointype.CoinType
		expectedMin    int64
	}{
		{
			name:           "VAR transaction 250 bytes",
			serializedSize: 250,
			coinType:       cointype.CoinTypeVAR,
			expectedMin:    2500, // (250 * 10000) / 1000 = 2500 atoms
		},
		{
			name:           "SKA transaction 250 bytes",
			serializedSize: 250,
			coinType:       cointype.CoinType(1), // SKA-1
			expectedMin:    1000000000000000000,  // SKA fee rate 4e18/KB: (250 * 4e18) / 1000 = 1e18 atoms
		},
		{
			name:           "Large VAR transaction 1000 bytes",
			serializedSize: 1000,
			coinType:       cointype.CoinTypeVAR,
			expectedMin:    10000, // (1000 * 10000) / 1000 = 10000 atoms
		},
		{
			name:           "Large SKA transaction 1000 bytes",
			serializedSize: 1000,
			coinType:       cointype.CoinType(1), // SKA-1
			expectedMin:    4000000000000000000,  // SKA fee rate 4e18/KB: (1000 * 4e18) / 1000 = 4e18 atoms
		},
		{
			name:           "Unknown coin type defaults to VAR",
			serializedSize: 500,
			coinType:       cointype.CoinType(99),
			expectedMin:    5000, // Should default to VAR: (500 * 10000) / 1000 = 5000 atoms
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			minFee := calc.CalculateMinFee(test.serializedSize, test.coinType)
			expectedBig := big.NewInt(test.expectedMin)
			if minFee.Cmp(expectedBig) != 0 {
				t.Errorf("Expected minimum fee %s, got %s", expectedBig.String(), minFee.String())
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
		coinType            cointype.CoinType
		targetConfirmations int
		expectError         bool
	}{
		{
			name:                "VAR next block",
			coinType:            cointype.CoinTypeVAR,
			targetConfirmations: 1,
			expectError:         false,
		},
		{
			name:                "SKA fast confirmation",
			coinType:            cointype.CoinType(1), // SKA-1
			targetConfirmations: 3,
			expectError:         false,
		},
		{
			name:                "VAR normal confirmation",
			coinType:            cointype.CoinTypeVAR,
			targetConfirmations: 6,
			expectError:         false,
		},
		{
			name:                "Unknown coin type",
			coinType:            cointype.CoinType(99),
			targetConfirmations: 1,
			expectError:         true,
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

			if feeRate.Sign() <= 0 {
				t.Error("Expected positive fee rate")
			}

			// Next block should have higher fee than normal confirmation
			if test.targetConfirmations == 1 {
				normalRate, _ := calc.EstimateFeeRate(test.coinType, 6)
				if feeRate.Cmp(normalRate) <= 0 {
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
	initialStats, err := calc.GetFeeStats(cointype.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get initial fee stats: %v", err)
	}
	initialMultiplier := initialStats.DynamicFeeMultiplier

	// Update with high utilization
	calc.UpdateUtilization(cointype.CoinTypeVAR, 150, 50000, 0.95) // 95% block space used, 150 pending txs

	// Get updated fee rate
	updatedStats, err := calc.GetFeeStats(cointype.CoinTypeVAR)
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
	calc.RecordTransactionFee(cointype.CoinTypeVAR, 5000, 250, true)  // 20000 atoms/KB
	calc.RecordTransactionFee(cointype.CoinTypeVAR, 7500, 250, true)  // 30000 atoms/KB
	calc.RecordTransactionFee(cointype.CoinTypeVAR, 10000, 250, true) // 40000 atoms/KB

	// Get fee stats
	stats, err := calc.GetFeeStats(cointype.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get fee stats: %v", err)
	}

	// Check that fee rates are calculated
	if stats.FastFee == nil || stats.FastFee.Sign() == 0 {
		t.Error("Expected non-zero fast fee")
	}
	if stats.NormalFee == nil || stats.NormalFee.Sign() == 0 {
		t.Error("Expected non-zero normal fee")
	}
	if stats.SlowFee == nil || stats.SlowFee.Sign() == 0 {
		t.Error("Expected non-zero slow fee")
	}

	// Fast fee should be >= normal fee >= slow fee
	if stats.FastFee.Cmp(stats.NormalFee) < 0 {
		t.Error("Fast fee should be >= normal fee")
	}
	if stats.NormalFee.Cmp(stats.SlowFee) < 0 {
		t.Error("Normal fee should be >= slow fee")
	}
}

// TestValidateTransactionFees tests transaction fee validation
func TestValidateTransactionFees(t *testing.T) {
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)

	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)

	tests := []struct {
		name           string
		txFee          int64
		serializedSize int64
		coinType       cointype.CoinType
		allowHighFees  bool
		expectError    bool
		errorContains  string
	}{
		{
			name:           "VAR sufficient fee",
			txFee:          3000,
			serializedSize: 250,
			coinType:       cointype.CoinTypeVAR,
			allowHighFees:  false,
			expectError:    false,
		},
		{
			name:           "VAR insufficient fee",
			txFee:          1000,
			serializedSize: 250,
			coinType:       cointype.CoinTypeVAR,
			allowHighFees:  false,
			expectError:    true,
			errorContains:  "insufficient fee",
		},
		{
			name:           "SKA sufficient fee",
			txFee:          2000000000000000000, // 2 SKA - above min fee of 1 SKA for 250 bytes
			serializedSize: 250,
			coinType:       cointype.CoinType(1), // SKA-1
			allowHighFees:  false,
			expectError:    false,
		},
		{
			name:           "SKA insufficient fee",
			txFee:          100000000000000000, // 0.1 SKA - below min fee of 1 SKA for 250 bytes
			serializedSize: 250,
			coinType:       cointype.CoinType(1), // SKA-1 - min fee is (250*4e18)/1000=1e18 atoms
			allowHighFees:  false,
			expectError:    true,
			errorContains:  "insufficient fee",
		},
		{
			name:           "VAR excessive fee allowed",
			txFee:          1000000, // Very high fee
			serializedSize: 250,
			coinType:       cointype.CoinTypeVAR,
			allowHighFees:  true,
			expectError:    false,
		},
		{
			name:           "VAR excessive fee rejected",
			txFee:          2000000, // Very high fee (exceeds max of 1000000)
			serializedSize: 250,
			coinType:       cointype.CoinTypeVAR,
			allowHighFees:  false,
			expectError:    true,
			errorContains:  "fee too high",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := calc.ValidateTransactionFees(big.NewInt(test.txFee), test.serializedSize,
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
	baselineFee := calc.CalculateMinFee(250, cointype.CoinTypeVAR)

	// Simulate high network utilization
	calc.UpdateUtilization(cointype.CoinTypeVAR, 200, 100000, 0.9) // High utilization

	// Fee should increase
	highUtilFee := calc.CalculateMinFee(250, cointype.CoinTypeVAR)
	if highUtilFee.Cmp(baselineFee) <= 0 {
		t.Error("Expected fee to increase with high utilization")
	}

	// Simulate low network utilization
	calc.UpdateUtilization(cointype.CoinTypeVAR, 5, 1000, 0.1) // Low utilization

	// Allow time for smoothing
	time.Sleep(time.Millisecond * 10)
	calc.UpdateUtilization(cointype.CoinTypeVAR, 5, 1000, 0.1) // Update again

	lowUtilFee := calc.CalculateMinFee(250, cointype.CoinTypeVAR)
	if lowUtilFee.Cmp(highUtilFee) >= 0 {
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
		calc.RecordTransactionFee(cointype.CoinTypeVAR, fee, 250, true)
	}

	stats, err := calc.GetFeeStats(cointype.CoinTypeVAR)
	if err != nil {
		t.Fatalf("Failed to get fee stats: %v", err)
	}

	// Check basic consistency
	if stats.DynamicFeeMultiplier < 0.1 || stats.DynamicFeeMultiplier > 20.0 {
		t.Errorf("Dynamic fee multiplier %f is out of reasonable range", stats.DynamicFeeMultiplier)
	}

	if stats.MinRelayFee == nil || stats.MinRelayFee.Sign() <= 0 {
		t.Error("Min relay fee should be positive")
	}

	if stats.MaxFeeRate == nil || stats.MaxFeeRate.Cmp(stats.MinRelayFee) <= 0 {
		t.Error("Max fee rate should be greater than min relay fee")
	}

	if stats.LastUpdated.IsZero() {
		t.Error("Last updated time should be set")
	}
}

// TestSKASpecificFeeBehavior tests SKA-specific fee behavior
func TestSKASpecificFeeBehavior(t *testing.T) {
	params := chaincfg.SimNetParams()
	// Set custom SKA fee rate in per-coin config
	if config, ok := params.SKACoins[cointype.CoinType(1)]; ok {
		config.MinRelayTxFee = big.NewInt(500) // Custom SKA fee rate
	}
	defaultMinRelayFee := dcrutil.Amount(1e4)

	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)

	// SKA should use custom fee rate
	skaFee := calc.CalculateMinFee(1000, cointype.CoinType(1)) // 1KB transaction, SKA-1
	expectedSKAFee := big.NewInt(500)                          // Should use SKACoinConfig.MinRelayTxFee

	if skaFee.Cmp(expectedSKAFee) != 0 {
		t.Errorf("Expected SKA fee %s, got %s", expectedSKAFee.String(), skaFee.String())
	}

	// VAR should still use default
	varFee := calc.CalculateMinFee(1000, cointype.CoinTypeVAR)
	expectedVARFee := big.NewInt(10000) // Should use defaultMinRelayFee

	if varFee.Cmp(expectedVARFee) != 0 {
		t.Errorf("Expected VAR fee %s, got %s", expectedVARFee.String(), varFee.String())
	}

	// SKA fees should be lower than VAR fees for same transaction size
	if skaFee.Cmp(varFee) >= 0 {
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

// TestInitializeActiveSKACoinsFromConfig tests that initializeDefaultFeeRates correctly
// reads active SKA coins from the chain configuration and initializes them
func TestInitializeActiveSKACoinsFromConfig(t *testing.T) {
	// Use SimNetParams which has predefined SKA coins
	params := chaincfg.SimNetParams()
	defaultMinRelayFee := dcrutil.Amount(1e4)

	// Create a new fee calculator
	calc := NewCoinTypeFeeCalculator(params, defaultMinRelayFee)

	// Check that VAR is always initialized
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	if _, exists := calc.feeRates[cointype.CoinTypeVAR]; !exists {
		t.Error("VAR coin type should always be initialized")
	}

	if _, exists := calc.utilizationStats[cointype.CoinTypeVAR]; !exists {
		t.Error("VAR utilization stats should always be initialized")
	}

	// Check that active SKA coins from config are initialized
	// Based on simnetparams.go: SKA-1 is active (Active: true), SKA-2 is inactive (Active: false)
	expectedActiveSKACoins := []cointype.CoinType{}
	for coinType, config := range params.SKACoins {
		if config.Active {
			expectedActiveSKACoins = append(expectedActiveSKACoins, coinType)
		}
	}

	t.Logf("Expected active SKA coins: %v", expectedActiveSKACoins)

	// Verify that each expected active SKA coin is initialized
	for _, coinType := range expectedActiveSKACoins {
		if _, exists := calc.feeRates[coinType]; !exists {
			t.Errorf("Active SKA coin type %d should be initialized with fee rates", coinType)
		}

		if _, exists := calc.utilizationStats[coinType]; !exists {
			t.Errorf("Active SKA coin type %d should be initialized with utilization stats", coinType)
		}

		// Verify the fee rate uses expected SKA defaults from per-coin config
		feeRate := calc.feeRates[coinType]

		// SKA uses SKAMinRelayFee (big.Int), not MinRelayFee (int64)
		// MinRelayFee should be 0 for SKA since it's not used
		if feeRate.MinRelayFee != 0 {
			t.Errorf("SKA coin type %d expected MinRelayFee 0 (unused), got %d",
				coinType, feeRate.MinRelayFee)
		}

		// Verify SKAMinRelayFee is properly set from config
		if config, ok := params.SKACoins[coinType]; ok && config.MinRelayTxFee != nil && config.MinRelayTxFee.Sign() > 0 {
			if feeRate.SKAMinRelayFee == nil || feeRate.SKAMinRelayFee.Cmp(config.MinRelayTxFee) != 0 {
				t.Errorf("SKA coin type %d expected SKAMinRelayFee %s, got %s",
					coinType, config.MinRelayTxFee.String(), feeRate.SKAMinRelayFee.String())
			}
		} else if feeRate.SKAMinRelayFee == nil {
			t.Errorf("SKA coin type %d should have SKAMinRelayFee set", coinType)
		}

		if feeRate.DynamicFeeMultiplier != 1.0 {
			t.Errorf("SKA coin type %d expected dynamic fee multiplier 1.0, got %f",
				coinType, feeRate.DynamicFeeMultiplier)
		}

		// MaxFeeRate (int64) is not used for SKA - SKAMaxFeeRate (big.Int) is used instead
		if feeRate.MaxFeeRate != 0 {
			t.Errorf("SKA coin type %d expected max fee rate 0 (unused), got %d",
				coinType, feeRate.MaxFeeRate)
		}

		// Verify SKAMaxFeeRate is properly set (2500x min fee)
		if feeRate.SKAMaxFeeRate == nil {
			t.Errorf("SKA coin type %d should have SKAMaxFeeRate set", coinType)
		}
	}

	// Check that inactive SKA coins are NOT initialized initially
	for coinType, config := range params.SKACoins {
		if !config.Active {
			if _, exists := calc.feeRates[coinType]; exists {
				t.Errorf("Inactive SKA coin type %d should not be initialized initially", coinType)
			}
		}
	}

	t.Logf("Successfully verified %d active SKA coins are initialized from config", len(expectedActiveSKACoins))
}
