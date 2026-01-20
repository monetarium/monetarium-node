// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cointype

import (
	"errors"
	"math/big"
	"testing"
)

// TestSKAAmountCreation tests various ways to create SKAAmount instances.
func TestSKAAmountCreation(t *testing.T) {
	tests := []struct {
		name     string
		create   func() SKAAmount
		expected string
	}{
		{
			name:     "Zero",
			create:   Zero,
			expected: "0",
		},
		{
			name:     "FromInt64 positive",
			create:   func() SKAAmount { return SKAAmountFromInt64(12345) },
			expected: "12345",
		},
		{
			name:     "FromInt64 negative",
			create:   func() SKAAmount { return SKAAmountFromInt64(-12345) },
			expected: "-12345",
		},
		{
			name:     "FromInt64 zero",
			create:   func() SKAAmount { return SKAAmountFromInt64(0) },
			expected: "0",
		},
		{
			name: "NewSKAAmount from big.Int",
			create: func() SKAAmount {
				return NewSKAAmount(big.NewInt(999999))
			},
			expected: "999999",
		},
		{
			name: "NewSKAAmount from nil",
			create: func() SKAAmount {
				return NewSKAAmount(nil)
			},
			expected: "0",
		},
		{
			name: "FromCoins",
			create: func() SKAAmount {
				return SKAAmountFromCoins(1)
			},
			expected: "1000000000000000000", // 10^18
		},
		{
			name: "FromCoins 100",
			create: func() SKAAmount {
				return SKAAmountFromCoins(100)
			},
			expected: "100000000000000000000", // 100 * 10^18
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.create()
			if result.String() != tt.expected {
				t.Errorf("got %s, want %s", result.String(), tt.expected)
			}
		})
	}
}

// TestSKAAmountFromString tests parsing string representations.
func TestSKAAmountFromString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "positive number",
			input:    "123456789012345678901234567890",
			expected: "123456789012345678901234567890",
		},
		{
			name:     "negative number",
			input:    "-123456789",
			expected: "-123456789",
		},
		{
			name:     "zero",
			input:    "0",
			expected: "0",
		},
		{
			name:      "empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "invalid string",
			input:     "abc",
			expectErr: true,
		},
		{
			name:      "float string",
			input:     "123.456",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SKAAmountFromString(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result.String() != tt.expected {
				t.Errorf("got %s, want %s", result.String(), tt.expected)
			}
		})
	}
}

// TestSKAAmountArithmetic tests Add, Sub, Mul, Div operations.
func TestSKAAmountArithmetic(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		b := SKAAmountFromInt64(50)
		result := a.Add(b)
		if result.String() != "150" {
			t.Errorf("100 + 50 = %s, want 150", result.String())
		}
	})

	t.Run("Add large numbers", func(t *testing.T) {
		// 10^30 + 10^30 = 2*10^30
		a, _ := SKAAmountFromString("1000000000000000000000000000000")
		b, _ := SKAAmountFromString("1000000000000000000000000000000")
		result := a.Add(b)
		expected := "2000000000000000000000000000000"
		if result.String() != expected {
			t.Errorf("got %s, want %s", result.String(), expected)
		}
	})

	t.Run("Sub positive result", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		b := SKAAmountFromInt64(30)
		result := a.Sub(b)
		if result.String() != "70" {
			t.Errorf("100 - 30 = %s, want 70", result.String())
		}
	})

	t.Run("Sub negative result", func(t *testing.T) {
		a := SKAAmountFromInt64(30)
		b := SKAAmountFromInt64(100)
		result := a.Sub(b)
		if result.String() != "-70" {
			t.Errorf("30 - 100 = %s, want -70", result.String())
		}
	})

	t.Run("Mul", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		result := a.Mul(5)
		if result.String() != "500" {
			t.Errorf("100 * 5 = %s, want 500", result.String())
		}
	})

	t.Run("MulBig", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		n := big.NewInt(1000000000000)
		result := a.MulBig(n)
		if result.String() != "100000000000000" {
			t.Errorf("100 * 10^12 = %s, want 100000000000000", result.String())
		}
	})

	t.Run("Div", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		result := a.Div(3)
		if result.String() != "33" {
			t.Errorf("100 / 3 = %s, want 33", result.String())
		}
	})

	t.Run("DivBig", func(t *testing.T) {
		a, _ := SKAAmountFromString("100000000000000")
		n := big.NewInt(1000000000000)
		result := a.DivBig(n)
		if result.String() != "100" {
			t.Errorf("10^14 / 10^12 = %s, want 100", result.String())
		}
	})
}

// TestSKAAmountComparison tests Cmp, IsNegative, IsZero, IsPositive.
func TestSKAAmountComparison(t *testing.T) {
	positive := SKAAmountFromInt64(100)
	negative := SKAAmountFromInt64(-100)
	zero := Zero()

	t.Run("Cmp", func(t *testing.T) {
		if positive.Cmp(negative) != 1 {
			t.Error("positive should be greater than negative")
		}
		if negative.Cmp(positive) != -1 {
			t.Error("negative should be less than positive")
		}
		if positive.Cmp(SKAAmountFromInt64(100)) != 0 {
			t.Error("equal values should compare as 0")
		}
	})

	t.Run("IsNegative", func(t *testing.T) {
		if positive.IsNegative() {
			t.Error("positive should not be negative")
		}
		if !negative.IsNegative() {
			t.Error("negative should be negative")
		}
		if zero.IsNegative() {
			t.Error("zero should not be negative")
		}
	})

	t.Run("IsZero", func(t *testing.T) {
		if positive.IsZero() {
			t.Error("positive should not be zero")
		}
		if negative.IsZero() {
			t.Error("negative should not be zero")
		}
		if !zero.IsZero() {
			t.Error("zero should be zero")
		}
	})

	t.Run("IsPositive", func(t *testing.T) {
		if !positive.IsPositive() {
			t.Error("positive should be positive")
		}
		if negative.IsPositive() {
			t.Error("negative should not be positive")
		}
		if zero.IsPositive() {
			t.Error("zero should not be positive")
		}
	})

	t.Run("Sign", func(t *testing.T) {
		if positive.Sign() != 1 {
			t.Error("positive sign should be 1")
		}
		if negative.Sign() != -1 {
			t.Error("negative sign should be -1")
		}
		if zero.Sign() != 0 {
			t.Error("zero sign should be 0")
		}
	})
}

// TestSKAAmountBytes tests byte serialization and deserialization.
func TestSKAAmountBytes(t *testing.T) {
	t.Run("Bytes round-trip positive", func(t *testing.T) {
		original := SKAAmountFromInt64(123456789)
		bytes := original.Bytes()
		restored := SKAAmountFromBytes(bytes)
		if original.Cmp(restored) != 0 {
			t.Errorf("round-trip failed: got %s, want %s", restored.String(), original.String())
		}
	})

	t.Run("Bytes round-trip large number", func(t *testing.T) {
		// Test with a number larger than int64
		original, _ := SKAAmountFromString("123456789012345678901234567890")
		bytes := original.Bytes()
		restored := SKAAmountFromBytes(bytes)
		if original.Cmp(restored) != 0 {
			t.Errorf("round-trip failed: got %s, want %s", restored.String(), original.String())
		}
	})

	t.Run("Bytes zero", func(t *testing.T) {
		zero := Zero()
		bytes := zero.Bytes()
		if len(bytes) != 0 {
			t.Errorf("zero should serialize to empty bytes, got %d bytes", len(bytes))
		}
		restored := SKAAmountFromBytes(bytes)
		if !restored.IsZero() {
			t.Error("restored zero should be zero")
		}
	})

	t.Run("SignedBytes round-trip positive", func(t *testing.T) {
		original := SKAAmountFromInt64(123456789)
		bytes := original.SignedBytes()
		restored := SKAAmountFromSignedBytes(bytes)
		if original.Cmp(restored) != 0 {
			t.Errorf("round-trip failed: got %s, want %s", restored.String(), original.String())
		}
	})

	t.Run("SignedBytes round-trip negative", func(t *testing.T) {
		original := SKAAmountFromInt64(-123456789)
		bytes := original.SignedBytes()
		restored := SKAAmountFromSignedBytes(bytes)
		if original.Cmp(restored) != 0 {
			t.Errorf("round-trip failed: got %s, want %s", restored.String(), original.String())
		}
	})

	t.Run("SignedBytes zero", func(t *testing.T) {
		zero := Zero()
		bytes := zero.SignedBytes()
		if len(bytes) != 1 || bytes[0] != 0 {
			t.Errorf("zero should serialize to [0], got %v", bytes)
		}
		restored := SKAAmountFromSignedBytes(bytes)
		if !restored.IsZero() {
			t.Error("restored zero should be zero")
		}
	})
}

// TestSKAAmountInt64 tests conversion to int64.
func TestSKAAmountInt64(t *testing.T) {
	t.Run("fits in int64", func(t *testing.T) {
		original := SKAAmountFromInt64(9223372036854775807) // max int64
		val, err := original.Int64()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if val != 9223372036854775807 {
			t.Errorf("got %d, want max int64", val)
		}
	})

	t.Run("overflows int64", func(t *testing.T) {
		original, _ := SKAAmountFromString("9223372036854775808") // max int64 + 1
		_, err := original.Int64()
		if !errors.Is(err, ErrSKAAmountOverflow) {
			t.Errorf("expected overflow error, got %v", err)
		}
	})
}

// TestSKAAmountImmutability tests that operations don't modify originals.
func TestSKAAmountImmutability(t *testing.T) {
	t.Run("Add doesn't modify operands", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		b := SKAAmountFromInt64(50)
		_ = a.Add(b)
		if a.String() != "100" {
			t.Errorf("a was modified: got %s, want 100", a.String())
		}
		if b.String() != "50" {
			t.Errorf("b was modified: got %s, want 50", b.String())
		}
	})

	t.Run("BigInt returns copy", func(t *testing.T) {
		a := SKAAmountFromInt64(100)
		bigVal := a.BigInt()
		bigVal.SetInt64(999)
		if a.String() != "100" {
			t.Errorf("original was modified through BigInt: got %s, want 100", a.String())
		}
	})

	t.Run("NewSKAAmount copies input", func(t *testing.T) {
		input := big.NewInt(100)
		a := NewSKAAmount(input)
		input.SetInt64(999)
		if a.String() != "100" {
			t.Errorf("SKAAmount was modified through original input: got %s, want 100", a.String())
		}
	})
}

// TestSKAAmountLargeSupply tests handling of large SKA supply values.
// Note: MaxSupply is now per-config in SKACoinConfig, not a global constant.
func TestSKAAmountLargeSupply(t *testing.T) {
	// Example: 900 trillion coins * 10^18 atoms per coin = 9 * 10^32 atoms
	// This is what SKA-1 would have as MaxSupply in its config.
	largeSupply := SKAAmountFromCoinsBig(mustParseBigInt("900000000000000")) // 900 trillion

	expectedStr := "900000000000000000000000000000000" // 9 * 10^32

	if largeSupply.String() != expectedStr {
		t.Errorf("LargeSupply = %s, want %s", largeSupply.String(), expectedStr)
	}

	// Verify it exceeds int64 max
	_, err := largeSupply.Int64()
	if !errors.Is(err, ErrSKAAmountOverflow) {
		t.Error("Large SKA supply should overflow int64")
	}
}

// mustParseBigInt parses a string to big.Int, panics on failure.
func mustParseBigInt(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("invalid big.Int string: " + s)
	}
	return v
}

// TestSKAAmountAbs tests the Abs method.
func TestSKAAmountAbs(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{100, "100"},
		{-100, "100"},
		{0, "0"},
	}

	for _, tt := range tests {
		result := SKAAmountFromInt64(tt.input).Abs()
		if result.String() != tt.expected {
			t.Errorf("Abs(%d) = %s, want %s", tt.input, result.String(), tt.expected)
		}
	}
}

// TestSKAAmountNeg tests the Neg method.
func TestSKAAmountNeg(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{100, "-100"},
		{-100, "100"},
		{0, "0"},
	}

	for _, tt := range tests {
		result := SKAAmountFromInt64(tt.input).Neg()
		if result.String() != tt.expected {
			t.Errorf("Neg(%d) = %s, want %s", tt.input, result.String(), tt.expected)
		}
	}
}

// TestSKAAmountCopy tests the Copy method.
func TestSKAAmountCopy(t *testing.T) {
	original := SKAAmountFromInt64(12345)
	copied := original.Copy()

	// Verify values are equal
	if original.Cmp(copied) != 0 {
		t.Error("copy should equal original")
	}

	// Verify they're independent (modify copied's underlying bigint)
	copiedBig := copied.BigInt()
	copiedBig.SetInt64(99999)
	// Original should be unchanged
	if original.String() != "12345" {
		t.Errorf("original was modified: got %s, want 12345", original.String())
	}
}

// TestAtomsPerSKACoin verifies the atoms per SKA coin constant.
func TestAtomsPerSKACoin(t *testing.T) {
	// Test the exported variable directly
	expected := "1000000000000000000" // 10^18
	if AtomsPerSKACoin.String() != expected {
		t.Errorf("AtomsPerSKACoin = %s, want %s", AtomsPerSKACoin.String(), expected)
	}

	// Test GetAtomsPerSKACoin returns a copy
	atoms := GetAtomsPerSKACoin()
	if atoms.String() != expected {
		t.Errorf("GetAtomsPerSKACoin = %s, want %s", atoms.String(), expected)
	}

	// Verify it's a copy (modifying should not affect original)
	atoms.SetInt64(0)
	if AtomsPerSKACoin.String() != expected {
		t.Errorf("AtomsPerSKACoin was modified by GetAtomsPerSKACoin result")
	}
}

// TestAtomsPerCoinMethod tests the AtomsPerCoin method on CoinType.
func TestAtomsPerCoinMethod(t *testing.T) {
	// VAR returns AtomsPerVAR (1e8)
	if CoinTypeVAR.AtomsPerCoin() != int64(AtomsPerVAR) {
		t.Errorf("VAR AtomsPerCoin = %d, want %d", CoinTypeVAR.AtomsPerCoin(), int64(AtomsPerVAR))
	}

	// SKA returns 0 - use AtomsPerSKACoin (big.Int) instead
	ska1 := CoinType(1)
	if ska1.AtomsPerCoin() != 0 {
		t.Errorf("SKA-1 AtomsPerCoin = %d, want 0 (use AtomsPerSKACoin for SKA)", ska1.AtomsPerCoin())
	}
}

// TestUsesBigInt tests the UsesBigInt method on CoinType.
func TestUsesBigInt(t *testing.T) {
	if CoinTypeVAR.UsesBigInt() {
		t.Error("VAR should not use big.Int")
	}

	ska1 := CoinType(1)
	if !ska1.UsesBigInt() {
		t.Error("SKA-1 should use big.Int")
	}
}

// BenchmarkSKAAmountAdd benchmarks addition performance.
func BenchmarkSKAAmountAdd(b *testing.B) {
	a := SKAAmountFromInt64(123456789)
	c := SKAAmountFromInt64(987654321)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = a.Add(c)
	}
}

// BenchmarkSKAAmountFromBytes benchmarks deserialization.
func BenchmarkSKAAmountFromBytes(b *testing.B) {
	// Use a large amount similar to 900 trillion SKA
	amount := SKAAmountFromCoinsBig(mustParseBigInt("900000000000000"))
	bytes := amount.Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SKAAmountFromBytes(bytes)
	}
}
