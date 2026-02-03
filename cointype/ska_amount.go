// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cointype

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

var (
	// AtomsPerSKACoin is 10^18 - the number of atoms in one SKA coin.
	// This is the conversion factor between SKA coins and atoms (smallest unit).
	AtomsPerSKACoin = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	// zeroInt is a shared zero value for comparisons.
	zeroInt = big.NewInt(0)

	// ErrSKAAmountOverflow is returned when an SKA amount exceeds maximum capacity.
	ErrSKAAmountOverflow = errors.New("SKA amount overflow")

	// ErrSKAAmountNegative is returned when an SKA amount would be negative
	// in a context where negative values are not allowed.
	ErrSKAAmountNegative = errors.New("SKA amount cannot be negative")

	// ErrSKAAmountInvalidString is returned when parsing an invalid string.
	ErrSKAAmountInvalidString = errors.New("invalid SKA amount string")
)

// SKA amount constants for validation and fee calculations.
const (
	// MinSKADustAtoms is the minimum SKA amount (30 atoms) to avoid dust.
	// Outputs below this value will be rejected by the mempool.
	MinSKADustAtoms = 30

	// MinSKATransactionFeeAtoms is the minimum fee for SKA transactions (10 atoms).
	// This ensures safe distribution to all 5 stakers (at least 1 atom each after 50/50 split).
	MinSKATransactionFeeAtoms = 10

	// BytesPerKilobyte is used in fee rate calculations (atoms per KB).
	BytesPerKilobyte = 1000
)

var (
	// MinSKADustAmount is MinSKADustAtoms as *big.Int for comparison operations.
	MinSKADustAmount = big.NewInt(MinSKADustAtoms)

	// MinSKATransactionFee is MinSKATransactionFeeAtoms as *big.Int.
	MinSKATransactionFee = big.NewInt(MinSKATransactionFeeAtoms)

	// KilobyteInt is 1000 as *big.Int for fee calculations.
	KilobyteInt = big.NewInt(BytesPerKilobyte)
)

// SKAAmount represents an SKA coin amount using arbitrary precision arithmetic.
// It wraps math/big.Int to handle values up to 900 trillion coins with 18
// decimal precision. The value is stored in atoms (smallest indivisible unit).
//
// SKAAmount is designed to be immutable - all arithmetic operations return
// new SKAAmount instances rather than modifying the receiver.
type SKAAmount struct {
	value *big.Int
}

// NewSKAAmount creates a new SKAAmount from a big.Int value representing atoms.
// If the input is nil, a zero amount is returned.
// The input is copied to ensure immutability.
func NewSKAAmount(atoms *big.Int) SKAAmount {
	if atoms == nil {
		return SKAAmount{value: new(big.Int)}
	}
	return SKAAmount{value: new(big.Int).Set(atoms)}
}

// SKAAmountFromInt64 creates an SKAAmount from an int64 value in atoms.
// This is useful for small amounts and backward compatibility.
func SKAAmountFromInt64(atoms int64) SKAAmount {
	return SKAAmount{value: big.NewInt(atoms)}
}

// SKAAmountFromString parses a string representation of atoms into an SKAAmount.
// The string must be a valid decimal integer (may be negative for intermediate calculations).
// Returns an error if the string cannot be parsed.
func SKAAmountFromString(s string) (SKAAmount, error) {
	if s == "" {
		return Zero(), ErrSKAAmountInvalidString
	}

	value := new(big.Int)
	_, ok := value.SetString(s, 10)
	if !ok {
		return Zero(), fmt.Errorf("%w: %s", ErrSKAAmountInvalidString, s)
	}

	return SKAAmount{value: value}, nil
}

// SKAAmountFromBytes creates an SKAAmount from a big-endian byte slice.
// This is the inverse of Bytes() and is used for deserialization.
// An empty slice results in a zero amount.
// The bytes are interpreted as an unsigned integer.
func SKAAmountFromBytes(b []byte) SKAAmount {
	if len(b) == 0 {
		return Zero()
	}
	return SKAAmount{value: new(big.Int).SetBytes(b)}
}

// SKAAmountFromSignedBytes creates an SKAAmount from a signed big-endian byte slice.
// The first byte indicates the sign: 0 for positive/zero, 1 for negative.
// This is the inverse of SignedBytes() and handles negative values.
func SKAAmountFromSignedBytes(b []byte) SKAAmount {
	if len(b) == 0 {
		return Zero()
	}

	if len(b) == 1 {
		// Single byte with sign only means zero
		return Zero()
	}

	sign := b[0]
	magnitude := new(big.Int).SetBytes(b[1:])

	if sign == 1 {
		magnitude.Neg(magnitude)
	}

	return SKAAmount{value: magnitude}
}

// Zero returns a new SKAAmount with value zero.
func Zero() SKAAmount {
	return SKAAmount{value: new(big.Int)}
}

// Add returns a new SKAAmount that is the sum of a and b.
func (a SKAAmount) Add(b SKAAmount) SKAAmount {
	result := new(big.Int)
	aVal := a.value
	bVal := b.value
	if aVal == nil {
		aVal = zeroInt
	}
	if bVal == nil {
		bVal = zeroInt
	}
	result.Add(aVal, bVal)
	return SKAAmount{value: result}
}

// Sub returns a new SKAAmount that is a minus b.
// The result may be negative (for fee calculation purposes).
func (a SKAAmount) Sub(b SKAAmount) SKAAmount {
	result := new(big.Int)
	aVal := a.value
	bVal := b.value
	if aVal == nil {
		aVal = zeroInt
	}
	if bVal == nil {
		bVal = zeroInt
	}
	result.Sub(aVal, bVal)
	return SKAAmount{value: result}
}

// Mul returns a new SKAAmount that is a multiplied by n.
func (a SKAAmount) Mul(n int64) SKAAmount {
	result := new(big.Int)
	aVal := a.value
	if aVal == nil {
		aVal = zeroInt
	}
	result.Mul(aVal, big.NewInt(n))
	return SKAAmount{value: result}
}

// MulBig returns a new SKAAmount that is a multiplied by n (as big.Int).
func (a SKAAmount) MulBig(n *big.Int) SKAAmount {
	result := new(big.Int)
	aVal := a.value
	if aVal == nil {
		aVal = zeroInt
	}
	nVal := n
	if nVal == nil {
		nVal = zeroInt
	}
	result.Mul(aVal, nVal)
	return SKAAmount{value: result}
}

// Div returns a new SKAAmount that is a divided by n (integer division).
// Panics if n is zero.
func (a SKAAmount) Div(n int64) SKAAmount {
	if n == 0 {
		panic("division by zero")
	}
	result := new(big.Int)
	aVal := a.value
	if aVal == nil {
		aVal = zeroInt
	}
	result.Div(aVal, big.NewInt(n))
	return SKAAmount{value: result}
}

// DivBig returns a new SKAAmount that is a divided by n (as big.Int).
// Panics if n is zero.
func (a SKAAmount) DivBig(n *big.Int) SKAAmount {
	if n == nil || n.Sign() == 0 {
		panic("division by zero")
	}
	result := new(big.Int)
	aVal := a.value
	if aVal == nil {
		aVal = zeroInt
	}
	result.Div(aVal, n)
	return SKAAmount{value: result}
}

// Cmp compares a and b and returns:
//
//	-1 if a < b
//	 0 if a == b
//	+1 if a > b
func (a SKAAmount) Cmp(b SKAAmount) int {
	aVal := a.value
	bVal := b.value
	if aVal == nil {
		aVal = zeroInt
	}
	if bVal == nil {
		bVal = zeroInt
	}
	return aVal.Cmp(bVal)
}

// IsNegative returns true if the amount is less than zero.
func (a SKAAmount) IsNegative() bool {
	if a.value == nil {
		return false
	}
	return a.value.Sign() < 0
}

// IsZero returns true if the amount is exactly zero.
func (a SKAAmount) IsZero() bool {
	if a.value == nil {
		return true
	}
	return a.value.Sign() == 0
}

// IsPositive returns true if the amount is greater than zero.
func (a SKAAmount) IsPositive() bool {
	if a.value == nil {
		return false
	}
	return a.value.Sign() > 0
}

// Sign returns the sign of the amount:
//
//	-1 if a < 0
//	 0 if a == 0
//	+1 if a > 0
func (a SKAAmount) Sign() int {
	if a.value == nil {
		return 0
	}
	return a.value.Sign()
}

// Bytes returns the big-endian byte representation of the absolute value.
// This is used for wire protocol serialization of non-negative amounts.
// Returns an empty slice for zero values.
func (a SKAAmount) Bytes() []byte {
	if a.value == nil || a.value.Sign() == 0 {
		return nil
	}
	// Use absolute value for bytes
	absVal := new(big.Int).Abs(a.value)
	return absVal.Bytes()
}

// SignedBytes returns a byte representation that preserves the sign.
// Format: [sign byte][magnitude bytes...]
// Sign byte: 0 = positive/zero, 1 = negative
// This is used when negative intermediate values need to be stored.
func (a SKAAmount) SignedBytes() []byte {
	if a.value == nil || a.value.Sign() == 0 {
		return []byte{0} // Zero is represented as single 0 byte
	}

	absVal := new(big.Int).Abs(a.value)
	magnitude := absVal.Bytes()

	result := make([]byte, 1+len(magnitude))
	if a.value.Sign() < 0 {
		result[0] = 1 // Negative
	} else {
		result[0] = 0 // Positive
	}
	copy(result[1:], magnitude)

	return result
}

// BigInt returns a copy of the underlying big.Int value.
// The returned value is a copy to maintain immutability.
func (a SKAAmount) BigInt() *big.Int {
	if a.value == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(a.value)
}

// Int64 returns the amount as an int64 if it fits.
// Returns an error if the value overflows int64.
func (a SKAAmount) Int64() (int64, error) {
	if a.value == nil {
		return 0, nil
	}
	if !a.value.IsInt64() {
		return 0, ErrSKAAmountOverflow
	}
	return a.value.Int64(), nil
}

// String returns the string representation of the amount in atoms.
func (a SKAAmount) String() string {
	if a.value == nil {
		return "0"
	}
	return a.value.String()
}

// ToCoins converts the amount from atoms to whole coins as a string.
// This performs integer division and may lose precision.
// Use ToCoinsWithPrecision for formatted output.
func (a SKAAmount) ToCoins() *big.Int {
	if a.value == nil {
		return new(big.Int)
	}
	result := new(big.Int)
	result.Div(a.value, AtomsPerSKACoin)
	return result
}

// ToDecimalString converts atoms to a decimal string representation in coins.
// Uses the provided atomsPerCoin for conversion (e.g., 1e18 for SKA).
// Preserves full precision for display purposes.
// Example: atoms=1500000000000000000, atomsPerCoin=1e18 -> "1.5"
func (a SKAAmount) ToDecimalString(atomsPerCoin *big.Int) string {
	return AtomsToDecimalString(a.value, atomsPerCoin)
}

// AtomsToDecimalString converts a big.Int amount in atoms to a decimal string
// representation in coins. This is a standalone function for use with raw big.Int values.
// For SKAAmount values, use the ToDecimalString method instead.
func AtomsToDecimalString(atoms *big.Int, atomsPerCoin *big.Int) string {
	if atoms == nil {
		return "0"
	}
	if atomsPerCoin == nil || atomsPerCoin.Sign() == 0 {
		return atoms.String()
	}

	// Get number of decimal places from atomsPerCoin (e.g., 1e18 = 18 decimals)
	decimals := len(atomsPerCoin.String()) - 1

	// Handle negative amounts
	negative := atoms.Sign() < 0
	absAtoms := new(big.Int).Abs(atoms)

	// Get integer and fractional parts
	intPart := new(big.Int).Div(absAtoms, atomsPerCoin)
	fracPart := new(big.Int).Mod(absAtoms, atomsPerCoin)

	// Format the fractional part with leading zeros
	fracStr := fracPart.String()
	if len(fracStr) < decimals {
		fracStr = strings.Repeat("0", decimals-len(fracStr)) + fracStr
	}

	// Trim trailing zeros from fractional part
	fracStr = strings.TrimRight(fracStr, "0")

	var result string
	if fracStr == "" {
		result = intPart.String()
	} else {
		result = intPart.String() + "." + fracStr
	}

	if negative {
		result = "-" + result
	}

	return result
}

// Copy returns a deep copy of the SKAAmount.
func (a SKAAmount) Copy() SKAAmount {
	if a.value == nil {
		return Zero()
	}
	return SKAAmount{value: new(big.Int).Set(a.value)}
}

// Abs returns the absolute value of the amount.
func (a SKAAmount) Abs() SKAAmount {
	if a.value == nil {
		return Zero()
	}
	return SKAAmount{value: new(big.Int).Abs(a.value)}
}

// Neg returns the negation of the amount.
func (a SKAAmount) Neg() SKAAmount {
	if a.value == nil {
		return Zero()
	}
	return SKAAmount{value: new(big.Int).Neg(a.value)}
}

// GetAtomsPerSKACoin returns a copy of AtomsPerSKACoin (10^18).
// Use this when you need a mutable copy.
func GetAtomsPerSKACoin() *big.Int {
	return new(big.Int).Set(AtomsPerSKACoin)
}

// SKAAmountFromCoins creates an SKAAmount from a whole coin value.
// This multiplies by 10^18 to convert to atoms.
func SKAAmountFromCoins(coins int64) SKAAmount {
	value := new(big.Int).Mul(big.NewInt(coins), AtomsPerSKACoin)
	return SKAAmount{value: value}
}

// SKAAmountFromCoinsBig creates an SKAAmount from a big.Int coin value.
// This multiplies by 10^18 to convert to atoms.
func SKAAmountFromCoinsBig(coins *big.Int) SKAAmount {
	if coins == nil {
		return Zero()
	}
	value := new(big.Int).Mul(coins, AtomsPerSKACoin)
	return SKAAmount{value: value}
}

// MinSKADust returns the minimum dust amount as SKAAmount.
func MinSKADust() SKAAmount {
	return SKAAmountFromInt64(MinSKADustAtoms)
}

// MinSKAFee returns the minimum transaction fee as SKAAmount.
func MinSKAFee() SKAAmount {
	return SKAAmountFromInt64(MinSKATransactionFeeAtoms)
}

// IsSKADust returns true if the amount is below the dust threshold.
func (a SKAAmount) IsSKADust() bool {
	return a.Cmp(MinSKADust()) < 0
}
