// Copyright (c) 2013, 2014 The btcsuite developers
// Copyright (c) 2015 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcrutil

import (
	"errors"
	"math"
	"strconv"

	"github.com/monetarium/monetarium-node/cointype"
)

// AmountUnit describes a method of converting an Amount to something
// other than the base unit of a coin.  The value of the AmountUnit
// is the exponent component of the decadic multiple to convert from
// an amount in coins to an amount counted in atomic units.
type AmountUnit int

// These constants define various units used when describing a coin
// monetary amount.
const (
	AmountMegaCoin  AmountUnit = 6
	AmountKiloCoin  AmountUnit = 3
	AmountCoin      AmountUnit = 0
	AmountMilliCoin AmountUnit = -3
	AmountMicroCoin AmountUnit = -6
	AmountAtom      AmountUnit = -8
)

// String returns the unit as a string.  For recognized units, the SI
// prefix is used, or "Atom" for the base unit.  For all unrecognized
// units, "1eN VAR" is returned, where N is the AmountUnit.
func (u AmountUnit) String() string {
	switch u {
	case AmountMegaCoin:
		return "MVAR"
	case AmountKiloCoin:
		return "kVAR"
	case AmountCoin:
		return "VAR"
	case AmountMilliCoin:
		return "mVAR"
	case AmountMicroCoin:
		return "μVAR"
	case AmountAtom:
		return "Atom"
	default:
		return "1e" + strconv.FormatInt(int64(u), 10) + " VAR"
	}
}

// Amount represents the base coin monetary unit (colloquially referred
// to as an `Atom').  A single Amount is equal to 1e-8 of a coin.
type Amount int64

// round converts a floating point number, which may or may not be representable
// as an integer, to the Amount integer type by rounding to the nearest integer.
// This is performed by adding or subtracting 0.5 depending on the sign, and
// relying on integer truncation to round the value to the nearest Amount.
func round(f float64) Amount {
	if f < 0 {
		return Amount(f - 0.5)
	}
	return Amount(f + 0.5)
}

// NewAmount creates an Amount from a floating point value representing
// some value in the currency.  NewAmount errors if f is NaN or +-Infinity,
// but does not check that the amount is within the total amount of coins
// producible as f may not refer to an amount at a single moment in time.
//
// NewAmount is for specifically for converting VAR to Atoms (atomic units).
// For creating a new Amount with an int64 value which denotes a quantity of
// Atoms, do a simple type conversion from type int64 to Amount.
func NewAmount(f float64) (Amount, error) {
	return NewAmountForCoinType(f, cointype.CoinTypeVAR)
}

// NewAmountForCoinType creates an Amount from a floating point value for
// a specific coin type. This function handles the conversion from coins
// to atoms for both VAR and SKA.
func NewAmountForCoinType(f float64, coinType cointype.CoinType) (Amount, error) {
	// The amount is only considered invalid if it cannot be represented
	// as an integer type.  This may happen if f is NaN or +-Infinity.
	switch {
	case math.IsNaN(f):
		fallthrough
	case math.IsInf(f, 1):
		fallthrough
	case math.IsInf(f, -1):
		return 0, errors.New("invalid coin amount")
	}

	if !coinType.IsValid() {
		return 0, cointype.ErrInvalidCoinType
	}

	atomsPerCoin := coinType.AtomsPerCoin()
	return round(f * float64(atomsPerCoin)), nil
}

// ToUnit converts a monetary amount counted in coin base units to a
// floating point value representing an amount of coins.
func (a Amount) ToUnit(u AmountUnit) float64 {
	return float64(a) / math.Pow10(int(u+8))
}

// ToCoin is the equivalent of calling ToUnit with AmountCoin.
// This method assumes VAR for backward compatibility.
func (a Amount) ToCoin() float64 {
	return a.ToUnit(AmountCoin)
}

// ToVAR converts the amount to VAR coins as a floating point value.
func (a Amount) ToVAR() float64 {
	return float64(a) / cointype.AtomsPerVAR
}

// ToSKA converts the amount to SKA coins as a floating point value.
func (a Amount) ToSKA() float64 {
	return float64(a) / cointype.AtomsPerSKA
}

// ToCoinType converts the amount to coins for the specified coin type.
func (a Amount) ToCoinType(coinType cointype.CoinType) float64 {
	if !coinType.IsValid() {
		return 0
	}
	return float64(a) / float64(coinType.AtomsPerCoin())
}

// Format formats a monetary amount counted in coin base units as a
// string for a given unit.  The conversion will succeed for any unit,
// however, known units will be formatted with an appended label describing
// the units with SI notation, or "atom" for the base unit.
func (a Amount) Format(u AmountUnit) string {
	units := " " + u.String()
	return strconv.FormatFloat(a.ToUnit(u), 'f', -int(u+8), 64) + units
}

// String is the equivalent of calling Format with AmountCoin.
// This method assumes VAR for backward compatibility.
func (a Amount) String() string {
	return a.Format(AmountCoin)
}

// StringForCoinType formats the amount as a string for the specified coin type.
func (a Amount) StringForCoinType(coinType cointype.CoinType) string {
	if !coinType.IsValid() {
		return "0 Unknown"
	}
	value := a.ToCoinType(coinType)
	return strconv.FormatFloat(value, 'f', 8, 64) + " " + coinType.String()
}

// StringVAR formats the amount as a VAR string.
func (a Amount) StringVAR() string {
	return a.StringForCoinType(cointype.CoinTypeVAR)
}

// StringSKA formats the amount as a SKA string.
func (a Amount) StringSKA() string {
	return a.StringForCoinType(cointype.CoinType(1))
}

// MulF64 multiplies an Amount by a floating point value.  While this is not
// an operation that must typically be done by a full node or wallet, it is
// useful for services that build on top of Decred (for example, calculating
// a fee by multiplying by a percentage).
func (a Amount) MulF64(f float64) Amount {
	return round(float64(a) * f)
}

// AmountSorter implements sort.Interface to allow a slice of Amounts to
// be sorted.
type AmountSorter []Amount

// Len returns the number of Amounts in the slice.  It is part of the
// sort.Interface implementation.
func (s AmountSorter) Len() int {
	return len(s)
}

// Swap swaps the Amounts at the passed indices.  It is part of the
// sort.Interface implementation.
func (s AmountSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Less returns whether the Amount with index i should sort before the
// Amount with index j.  It is part of the sort.Interface
// implementation.
func (s AmountSorter) Less(i, j int) bool {
	return s[i] < s[j]
}
