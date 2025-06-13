// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcrutil

import (
	"testing"
)

// TestCoinTypeString tests the String method of CoinType.
func TestCoinTypeString(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected string
	}{
		{CoinTypeVAR, "VAR"},
		{CoinTypeSKA, "SKA"},
		{CoinType(99), "Unknown(99)"},
	}

	for i, test := range tests {
		result := test.coinType.String()
		if result != test.expected {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, result)
		}
	}
}

// TestCoinTypeIsValid tests the IsValid method of CoinType.
func TestCoinTypeIsValid(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected bool
	}{
		{CoinTypeVAR, true},
		{CoinTypeSKA, true},
		{CoinType(2), false},
		{CoinType(99), false},
	}

	for i, test := range tests {
		result := test.coinType.IsValid()
		if result != test.expected {
			t.Errorf("Test %d: expected %t, got %t", i, test.expected, result)
		}
	}
}

// TestCoinTypeAtomsPerCoin tests the AtomsPerCoin method.
func TestCoinTypeAtomsPerCoin(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected int64
	}{
		{CoinTypeVAR, AtomsPerVAR},
		{CoinTypeSKA, AtomsPerSKA},
		{CoinType(99), 0},
	}

	for i, test := range tests {
		result := test.coinType.AtomsPerCoin()
		if result != test.expected {
			t.Errorf("Test %d: expected %d, got %d", i, test.expected, result)
		}
	}
}

// TestCoinTypeMaxAtoms tests the MaxAtoms method.
func TestCoinTypeMaxAtoms(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected int64
	}{
		{CoinTypeVAR, MaxVARAtoms},
		{CoinTypeSKA, MaxSKAAtoms},
		{CoinType(99), 0},
	}

	for i, test := range tests {
		result := test.coinType.MaxAtoms()
		if result != test.expected {
			t.Errorf("Test %d: expected %d, got %d", i, test.expected, result)
		}
	}
}

// TestCoinTypeMaxAmount tests the MaxAmount method.
func TestCoinTypeMaxAmount(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected Amount
	}{
		{CoinTypeVAR, MaxVARAmount},
		{CoinTypeSKA, MaxSKAAmount},
		{CoinType(99), 0},
	}

	for i, test := range tests {
		result := test.coinType.MaxAmount()
		if result != test.expected {
			t.Errorf("Test %d: expected %d, got %d", i, test.expected, result)
		}
	}
}

// TestParseCoinType tests the ParseCoinType function.
func TestParseCoinType(t *testing.T) {
	tests := []struct {
		input       string
		expected    CoinType
		shouldError bool
	}{
		{"VAR", CoinTypeVAR, false},
		{"var", CoinTypeVAR, false},
		{"SKA", CoinTypeSKA, false},
		{"ska", CoinTypeSKA, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for i, test := range tests {
		result, err := ParseCoinType(test.input)
		if test.shouldError {
			if err == nil {
				t.Errorf("Test %d: expected error but got none", i)
			}
		} else {
			if err != nil {
				t.Errorf("Test %d: unexpected error: %v", i, err)
			}
			if result != test.expected {
				t.Errorf("Test %d: expected %d, got %d", i, test.expected, result)
			}
		}
	}
}

// TestValidateCoinType tests the ValidateCoinType function.
func TestValidateCoinType(t *testing.T) {
	tests := []struct {
		coinType    CoinType
		shouldError bool
	}{
		{CoinTypeVAR, false},
		{CoinTypeSKA, false},
		{CoinType(2), true},
		{CoinType(99), true},
	}

	for i, test := range tests {
		err := ValidateCoinType(test.coinType)
		if test.shouldError {
			if err == nil {
				t.Errorf("Test %d: expected error but got none", i)
			}
		} else {
			if err != nil {
				t.Errorf("Test %d: unexpected error: %v", i, err)
			}
		}
	}
}
