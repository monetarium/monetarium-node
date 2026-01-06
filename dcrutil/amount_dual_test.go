// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcrutil

import (
	"math"
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
)

// TestNewAmountForCoinType tests the NewAmountForCoinType function.
func TestNewAmountForCoinType(t *testing.T) {
	tests := []struct {
		name        string
		amount      float64
		coinType    cointype.CoinType
		expected    Amount
		shouldError bool
	}{
		{"VAR 1.0", 1.0, cointype.CoinTypeVAR, Amount(cointype.AtomsPerVAR), false},
		{"SKA 1.0", 1.0, cointype.CoinType(1), Amount(cointype.AtomsPerSKA), false},
		{"VAR 0.5", 0.5, cointype.CoinTypeVAR, Amount(cointype.AtomsPerVAR / 2), false},
		{"SKA 0.5", 0.5, cointype.CoinType(1), Amount(cointype.AtomsPerSKA / 2), false},
		{"VAR 0", 0.0, cointype.CoinTypeVAR, 0, false},
		{"SKA 0", 0.0, cointype.CoinType(1), 0, false},
		{"Valid SKA-99 coin type", 1.0, cointype.CoinType(99), Amount(cointype.AtomsPerSKA), false},
		{"NaN", math.NaN(), cointype.CoinTypeVAR, 0, true},
		{"Positive infinity", math.Inf(1), cointype.CoinTypeVAR, 0, true},
		{"Negative infinity", math.Inf(-1), cointype.CoinTypeVAR, 0, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := NewAmountForCoinType(test.amount, test.coinType)
			if test.shouldError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != test.expected {
					t.Errorf("Expected %d, got %d", test.expected, result)
				}
			}
		})
	}
}

// TestAmountToCoinType tests the ToCoinType method.
func TestAmountToCoinType(t *testing.T) {
	tests := []struct {
		name     string
		amount   Amount
		coinType cointype.CoinType
		expected float64
	}{
		{"VAR 1 coin", Amount(cointype.AtomsPerVAR), cointype.CoinTypeVAR, 1.0},
		{"SKA 1 coin", Amount(cointype.AtomsPerSKA), cointype.CoinType(1), 1.0},
		{"VAR 0.5 coin", Amount(cointype.AtomsPerVAR / 2), cointype.CoinTypeVAR, 0.5},
		{"SKA 0.5 coin", Amount(cointype.AtomsPerSKA / 2), cointype.CoinType(1), 0.5},
		{"VAR 0", 0, cointype.CoinTypeVAR, 0.0},
		{"SKA 0", 0, cointype.CoinType(1), 0.0},
		{"Valid SKA-99 coin type", Amount(cointype.AtomsPerSKA), cointype.CoinType(99), 1.0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.amount.ToCoinType(test.coinType)
			if result != test.expected {
				t.Errorf("Expected %f, got %f", test.expected, result)
			}
		})
	}
}

// TestAmountToVAR tests the ToVAR method.
func TestAmountToVAR(t *testing.T) {
	tests := []struct {
		name     string
		amount   Amount
		expected float64
	}{
		{"1 VAR", Amount(cointype.AtomsPerVAR), 1.0},
		{"0.5 VAR", Amount(cointype.AtomsPerVAR / 2), 0.5},
		{"0 VAR", 0, 0.0},
		{"10 VAR", Amount(10 * cointype.AtomsPerVAR), 10.0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.amount.ToVAR()
			if result != test.expected {
				t.Errorf("Expected %f, got %f", test.expected, result)
			}
		})
	}
}

// TestAmountToSKA tests the ToSKA method.
func TestAmountToSKA(t *testing.T) {
	tests := []struct {
		name     string
		amount   Amount
		expected float64
	}{
		{"1 SKA", Amount(cointype.AtomsPerSKA), 1.0},
		{"0.5 SKA", Amount(cointype.AtomsPerSKA / 2), 0.5},
		{"0 SKA", 0, 0.0},
		{"10 SKA", Amount(10 * cointype.AtomsPerSKA), 10.0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.amount.ToSKA()
			if result != test.expected {
				t.Errorf("Expected %f, got %f", test.expected, result)
			}
		})
	}
}

// TestAmountStringForCoinType tests the StringForCoinType method.
func TestAmountStringForCoinType(t *testing.T) {
	tests := []struct {
		name     string
		amount   Amount
		coinType cointype.CoinType
		expected string
	}{
		{"VAR 1.0", Amount(cointype.AtomsPerVAR), cointype.CoinTypeVAR, "1.00000000 VAR"},
		{"SKA 1.0", Amount(cointype.AtomsPerSKA), cointype.CoinType(1), "1.00000000 SKA-1"},
		{"VAR 0.5", Amount(cointype.AtomsPerVAR / 2), cointype.CoinTypeVAR, "0.50000000 VAR"},
		{"SKA 0.5", Amount(cointype.AtomsPerSKA / 2), cointype.CoinType(1), "0.50000000 SKA-1"},
		{"VAR 0", 0, cointype.CoinTypeVAR, "0.00000000 VAR"},
		{"SKA 0", 0, cointype.CoinType(1), "0.00000000 SKA-1"},
		{"Valid SKA-99 coin type", Amount(cointype.AtomsPerSKA), cointype.CoinType(99), "1.00000000 SKA-99"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.amount.StringForCoinType(test.coinType)
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

// TestAmountStringVAR tests the StringVAR method.
func TestAmountStringVAR(t *testing.T) {
	amount := Amount(cointype.AtomsPerVAR)
	expected := "1.00000000 VAR"
	result := amount.StringVAR()
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// TestAmountStringSKA tests the StringSKA method.
func TestAmountStringSKA(t *testing.T) {
	amount := Amount(cointype.AtomsPerSKA)
	expected := "1.00000000 SKA-1"
	result := amount.StringSKA()
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// TestNewAmountBackwardCompatibility tests that NewAmount still works for VAR.
func TestNewAmountBackwardCompatibility(t *testing.T) {
	amount, err := NewAmount(1.0)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := Amount(cointype.AtomsPerVAR)
	if amount != expected {
		t.Errorf("Expected %d, got %d", expected, amount)
	}

	// Should be equivalent to VAR
	varAmount, err := NewAmountForCoinType(1.0, cointype.CoinTypeVAR)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if amount != varAmount {
		t.Errorf("NewAmount and NewAmountForCoinType(VAR) should be equivalent")
	}
}
