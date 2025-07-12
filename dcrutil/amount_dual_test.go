// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcrutil

import (
	"math"
	"testing"
)

// TestNewAmountForCoinType tests the NewAmountForCoinType function.
func TestNewAmountForCoinType(t *testing.T) {
	tests := []struct {
		name        string
		amount      float64
		coinType    CoinType
		expected    Amount
		shouldError bool
	}{
		{"VAR 1.0", 1.0, CoinTypeVAR, Amount(AtomsPerVAR), false},
		{"SKA 1.0", 1.0, CoinTypeSKA, Amount(AtomsPerSKA), false},
		{"VAR 0.5", 0.5, CoinTypeVAR, Amount(AtomsPerVAR / 2), false},
		{"SKA 0.5", 0.5, CoinTypeSKA, Amount(AtomsPerSKA / 2), false},
		{"VAR 0", 0.0, CoinTypeVAR, 0, false},
		{"SKA 0", 0.0, CoinTypeSKA, 0, false},
		{"Valid SKA-99 coin type", 1.0, CoinType(99), Amount(AtomsPerSKA), false},
		{"NaN", math.NaN(), CoinTypeVAR, 0, true},
		{"Positive infinity", math.Inf(1), CoinTypeVAR, 0, true},
		{"Negative infinity", math.Inf(-1), CoinTypeVAR, 0, true},
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
		coinType CoinType
		expected float64
	}{
		{"VAR 1 coin", Amount(AtomsPerVAR), CoinTypeVAR, 1.0},
		{"SKA 1 coin", Amount(AtomsPerSKA), CoinTypeSKA, 1.0},
		{"VAR 0.5 coin", Amount(AtomsPerVAR / 2), CoinTypeVAR, 0.5},
		{"SKA 0.5 coin", Amount(AtomsPerSKA / 2), CoinTypeSKA, 0.5},
		{"VAR 0", 0, CoinTypeVAR, 0.0},
		{"SKA 0", 0, CoinTypeSKA, 0.0},
		{"Valid SKA-99 coin type", Amount(AtomsPerSKA), CoinType(99), 1.0},
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
		{"1 VAR", Amount(AtomsPerVAR), 1.0},
		{"0.5 VAR", Amount(AtomsPerVAR / 2), 0.5},
		{"0 VAR", 0, 0.0},
		{"10 VAR", Amount(10 * AtomsPerVAR), 10.0},
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
		{"1 SKA", Amount(AtomsPerSKA), 1.0},
		{"0.5 SKA", Amount(AtomsPerSKA / 2), 0.5},
		{"0 SKA", 0, 0.0},
		{"10 SKA", Amount(10 * AtomsPerSKA), 10.0},
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
		coinType CoinType
		expected string
	}{
		{"VAR 1.0", Amount(AtomsPerVAR), CoinTypeVAR, "1.00000000 VAR"},
		{"SKA 1.0", Amount(AtomsPerSKA), CoinTypeSKA, "1.00000000 SKA"},
		{"VAR 0.5", Amount(AtomsPerVAR / 2), CoinTypeVAR, "0.50000000 VAR"},
		{"SKA 0.5", Amount(AtomsPerSKA / 2), CoinTypeSKA, "0.50000000 SKA"},
		{"VAR 0", 0, CoinTypeVAR, "0.00000000 VAR"},
		{"SKA 0", 0, CoinTypeSKA, "0.00000000 SKA"},
		{"Valid SKA-99 coin type", Amount(AtomsPerSKA), CoinType(99), "1.00000000 SKA-99"},
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
	amount := Amount(AtomsPerVAR)
	expected := "1.00000000 VAR"
	result := amount.StringVAR()
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// TestAmountStringSKA tests the StringSKA method.
func TestAmountStringSKA(t *testing.T) {
	amount := Amount(AtomsPerSKA)
	expected := "1.00000000 SKA"
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
	expected := Amount(AtomsPerVAR)
	if amount != expected {
		t.Errorf("Expected %d, got %d", expected, amount)
	}

	// Should be equivalent to VAR
	varAmount, err := NewAmountForCoinType(1.0, CoinTypeVAR)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if amount != varAmount {
		t.Errorf("NewAmount and NewAmountForCoinType(VAR) should be equivalent")
	}
}
