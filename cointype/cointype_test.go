// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cointype

import (
	"testing"
)

func TestCoinType_IsVAR(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected bool
	}{
		{CoinTypeVAR, true},
		{CoinType(1), false},
		{CoinType(2), false},
		{CoinType(255), false},
	}

	for _, test := range tests {
		result := test.coinType.IsVAR()
		if result != test.expected {
			t.Errorf("CoinType(%d).IsVAR() = %t, expected %t",
				test.coinType, result, test.expected)
		}
	}
}

func TestCoinType_IsSKA(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected bool
	}{
		{CoinTypeVAR, false},
		{CoinType(1), true},
		{CoinType(2), true},
		{CoinType(255), true},
	}

	for _, test := range tests {
		result := test.coinType.IsSKA()
		if result != test.expected {
			t.Errorf("CoinType(%d).IsSKA() = %t, expected %t",
				test.coinType, result, test.expected)
		}
	}
}

func TestCoinType_IsValid(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected bool
	}{
		{CoinTypeVAR, true},
		{CoinType(1), true},
		{CoinType(127), true},
		{CoinType(255), true},
	}

	for _, test := range tests {
		result := test.coinType.IsValid()
		if result != test.expected {
			t.Errorf("CoinType(%d).IsValid() = %t, expected %t",
				test.coinType, result, test.expected)
		}
	}
}

func TestCoinType_String(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected string
	}{
		{CoinTypeVAR, "VAR"},
		{CoinType(1), "SKA-1"},
		{CoinType(2), "SKA-2"},
		{CoinType(42), "SKA-42"},
		{CoinType(255), "SKA-255"},
	}

	for _, test := range tests {
		result := test.coinType.String()
		if result != test.expected {
			t.Errorf("CoinType(%d).String() = %q, expected %q",
				test.coinType, result, test.expected)
		}
	}
}

func TestCoinType_AtomsPerCoin(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected int64
	}{
		{CoinTypeVAR, AtomsPerVAR},
		// SKA returns 0 - use AtomsPerSKACoin (big.Int) or SKACoinConfig.AtomsPerCoin
		{CoinType(1), 0},
		{CoinType(2), 0},
		{CoinType(255), 0},
	}

	for _, test := range tests {
		result := test.coinType.AtomsPerCoin()
		if result != test.expected {
			t.Errorf("CoinType(%d).AtomsPerCoin() = %d, expected %d",
				test.coinType, result, test.expected)
		}
	}
}

func TestCoinType_MaxAtoms(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected int64
	}{
		{CoinTypeVAR, MaxVARAtoms},
		// SKA returns 0 - max supply is per-config (big.Int) in SKACoinConfig.MaxSupply
		{CoinType(1), 0},
		{CoinType(2), 0},
		{CoinType(255), 0},
	}

	for _, test := range tests {
		result := test.coinType.MaxAtoms()
		if result != test.expected {
			t.Errorf("CoinType(%d).MaxAtoms() = %d, expected %d",
				test.coinType, result, test.expected)
		}
	}
}

func TestCoinType_MaxAmount(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected Amount
	}{
		{CoinTypeVAR, MaxVARAmount},
		// SKA returns 0 - max supply is per-config (big.Int) in SKACoinConfig.MaxSupply
		{CoinType(1), 0},
		{CoinType(2), 0},
		{CoinType(255), 0},
	}

	for _, test := range tests {
		result := test.coinType.MaxAmount()
		if result != test.expected {
			t.Errorf("CoinType(%d).MaxAmount() = %d, expected %d",
				test.coinType, result, test.expected)
		}
	}
}

func TestParseCoinType(t *testing.T) {
	tests := []struct {
		input       string
		expected    CoinType
		expectError bool
	}{
		{"VAR", CoinTypeVAR, false},
		{"var", CoinTypeVAR, false},
		{"SKA", CoinType(1), false},
		{"ska", CoinType(1), false},
		{"BTC", CoinType(0), true},
		{"invalid", CoinType(0), true},
		{"", CoinType(0), true},
	}

	for _, test := range tests {
		result, err := ParseCoinType(test.input)
		if test.expectError {
			if err == nil {
				t.Errorf("ParseCoinType(%q) expected error, got nil", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseCoinType(%q) unexpected error: %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("ParseCoinType(%q) = %d, expected %d",
					test.input, result, test.expected)
			}
		}
	}
}

func TestValidateCoinType(t *testing.T) {
	tests := []struct {
		coinType    CoinType
		expectError bool
	}{
		{CoinTypeVAR, false},
		{CoinType(1), false},
		{CoinType(127), false},
		{CoinType(255), false},
	}

	for _, test := range tests {
		err := ValidateCoinType(test.coinType)
		if test.expectError {
			if err == nil {
				t.Errorf("ValidateCoinType(%d) expected error, got nil", test.coinType)
			}
		} else {
			if err != nil {
				t.Errorf("ValidateCoinType(%d) unexpected error: %v", test.coinType, err)
			}
		}
	}
}
