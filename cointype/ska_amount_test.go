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
		restored, err := SKAAmountFromSignedBytes(bytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if original.Cmp(restored) != 0 {
			t.Errorf("round-trip failed: got %s, want %s", restored.String(), original.String())
		}
	})

	t.Run("SignedBytes round-trip negative", func(t *testing.T) {
		original := SKAAmountFromInt64(-123456789)
		bytes := original.SignedBytes()
		restored, err := SKAAmountFromSignedBytes(bytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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
		restored, err := SKAAmountFromSignedBytes(bytes)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !restored.IsZero() {
			t.Error("restored zero should be zero")
		}
	})

	t.Run("SignedBytes invalid sign byte 0x02 multi-byte", func(t *testing.T) {
		// Corrupt sign byte must surface as an error rather than silently
		// returning a positive amount.
		corrupt := []byte{0x02, 0x01, 0x00}
		got, err := SKAAmountFromSignedBytes(corrupt)
		if err == nil {
			t.Fatalf("expected error on sign byte 0x02, got %s", got.String())
		}
	})

	t.Run("SignedBytes invalid sign byte 0xFF multi-byte", func(t *testing.T) {
		corrupt := []byte{0xFF, 0xDE, 0xAD, 0xBE, 0xEF}
		got, err := SKAAmountFromSignedBytes(corrupt)
		if err == nil {
			t.Fatalf("expected error on sign byte 0xFF, got %s", got.String())
		}
	})

	t.Run("SignedBytes invalid sign byte 0x02 single-byte", func(t *testing.T) {
		// Single-byte input with a non-zero sign is also corruption.
		corrupt := []byte{0x02}
		got, err := SKAAmountFromSignedBytes(corrupt)
		if err == nil {
			t.Fatalf("expected error on single-byte sign 0x02, got %s", got.String())
		}
	})

	t.Run("SignedBytes empty stays zero with no error", func(t *testing.T) {
		got, err := SKAAmountFromSignedBytes(nil)
		if err != nil {
			t.Fatalf("unexpected error on empty bytes: %v", err)
		}
		if !got.IsZero() {
			t.Errorf("expected zero, got %s", got.String())
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
	// This is what SKA1 would have as MaxSupply in its config.
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
		t.Errorf("SKA1 AtomsPerCoin = %d, want 0 (use AtomsPerSKACoin for SKA)", ska1.AtomsPerCoin())
	}
}

// TestUsesBigInt tests the UsesBigInt method on CoinType.
func TestUsesBigInt(t *testing.T) {
	if CoinTypeVAR.UsesBigInt() {
		t.Error("VAR should not use big.Int")
	}

	ska1 := CoinType(1)
	if !ska1.UsesBigInt() {
		t.Error("SKA1 should use big.Int")
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

// TestDecimalStringToAtoms verifies the public decimal-string parser used by
// CLI tools and RPC handlers. Round-trips against AtomsToDecimalString and
// pins the precision-preservation contract for SKA (atoms > MaxInt64).
func TestDecimalStringToAtoms(t *testing.T) {
	atomsPerSKA := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	atomsPerVAR := big.NewInt(1e8)

	tests := []struct {
		name         string
		input        string
		atomsPerCoin *big.Int
		want         string
		wantErr      bool
	}{
		{"VAR whole", "1", atomsPerVAR, "100000000", false},
		{"VAR fraction", "1.5", atomsPerVAR, "150000000", false},
		{"VAR full precision", "0.12345678", atomsPerVAR, "12345678", false},
		{"VAR over-precise rejected", "0.123456789", atomsPerVAR, "", true},
		{"SKA whole", "1", atomsPerSKA, "1000000000000000000", false},
		{"SKA fraction", "1.5", atomsPerSKA, "1500000000000000000", false},
		{"SKA huge (above int64)", "50000000000", atomsPerSKA, "50000000000000000000000000000", false},
		{"SKA fractional huge", "12345678901234.123456", atomsPerSKA, "12345678901234123456000000000000", false},
		{"empty rejected", "", atomsPerSKA, "", true},
		{"negative rejected", "-1", atomsPerSKA, "", true},
		{"trailing space tolerated", " 2.5 ", atomsPerVAR, "250000000", false},
		{"too many dots rejected", "1.2.3", atomsPerVAR, "", true},
		{"non-numeric rejected", "abc", atomsPerVAR, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecimalStringToAtoms(tt.input, tt.atomsPerCoin)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got %s", got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got.String(), tt.want)
			}
		})
	}
}

// TestDecimalStringToAtomsRoundTrip verifies the parser is the inverse of
// AtomsToDecimalString for both VAR and SKA scales.
func TestDecimalStringToAtomsRoundTrip(t *testing.T) {
	atomsPerSKA := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	cases := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(1500000000000000000),
		mustParseBigInt("99999999999999999999999999999"),
	}
	for _, atoms := range cases {
		s := AtomsToDecimalString(atoms, atomsPerSKA)
		back, err := DecimalStringToAtoms(s, atomsPerSKA)
		if err != nil {
			t.Errorf("round-trip parse error for %s: %v", atoms.String(), err)
			continue
		}
		if back.Cmp(atoms) != 0 {
			t.Errorf("round-trip mismatch: got %s, want %s (via string %q)", back.String(), atoms.String(), s)
		}
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

// TestSKAAmountBigIntReturnsFreshCopy pins the contract that BigInt() returns
// a fresh *big.Int that does not alias the receiver's internal value. Multiple
// call sites in the wallet rely on this (e.g. wire-tx SKAValueIn assignments
// in monetarium-wallet/internal/rpc/jsonrpc/methods.go and wallet/multisig.go);
// if BigInt()'s contract is ever weakened to alias the inner pointer, those
// sites would silently corrupt wire-level transaction data.
func TestSKAAmountBigIntReturnsFreshCopy(t *testing.T) {
	t.Run("non-zero value", func(t *testing.T) {
		original := mustParseBigInt("12345678901234567890")
		amount := NewSKAAmount(original)

		got := amount.BigInt()
		if got.Cmp(original) != 0 {
			t.Fatalf("initial mismatch: got %s, want %s", got, original)
		}

		// Mutate the returned value.
		got.SetInt64(0)

		// Receiver must still hold the original value.
		again := amount.BigInt()
		if again.Cmp(original) != 0 {
			t.Fatalf("BigInt() aliases inner value: after mutation got %s, want %s",
				again, original)
		}
	})

	t.Run("zero value", func(t *testing.T) {
		var amount SKAAmount // zero value (internal value == nil)

		got := amount.BigInt()
		if got.Sign() != 0 {
			t.Fatalf("zero amount should yield zero big.Int, got %s", got)
		}

		// Mutate the returned value; receiver must still be zero.
		got.SetInt64(42)

		again := amount.BigInt()
		if again.Sign() != 0 {
			t.Fatalf("BigInt() aliases inner value for zero amount: got %s after mutation",
				again)
		}
	})
}
