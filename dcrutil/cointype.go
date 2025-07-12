// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package dcrutil

import (
	"errors"
	"fmt"
)

// CoinType represents the type of native coin in the Monetarium network.
// The network supports two native coins: VAR (Varta) and SKA (Skarb).
type CoinType uint8

const (
	// CoinTypeVAR represents Varta coins - the original mined cryptocurrency
	// that functions as network shares. VAR holders earn transaction fees
	// and can purchase PoS tickets.
	CoinTypeVAR CoinType = 0

	// CoinTypeSKA represents Skarb coins - pre-emitted asset-backed tokens
	// that represent ownership titles to real-world liquid assets.
	CoinTypeSKA CoinType = 1

	// CoinTypeMax defines the maximum valid coin type value.
	CoinTypeMax CoinType = 255
)

// Coin-specific constants for VAR (Varta)
const (
	// AtomsPerVAR is the number of atoms in one VAR coin.
	// This maintains compatibility with the original DCR atom system.
	AtomsPerVAR = 1e8

	// MaxVARAtoms is the maximum number of VAR atoms that can exist.
	// This follows the original DCR supply model.
	MaxVARAtoms = 21e6 * AtomsPerVAR

	// MaxVARAmount is the maximum VAR amount as an Amount type.
	MaxVARAmount = Amount(MaxVARAtoms)
)

// Coin-specific constants for SKA (Skarb)
const (
	// AtomsPerSKA is the number of atoms in one SKA coin.
	// Uses same precision as VAR for consistency.
	AtomsPerSKA = 1e8

	// MaxSKAAtoms is the maximum number of SKA atoms that can exist.
	// Set to 10 million SKA total supply.
	MaxSKAAtoms = 10e6 * AtomsPerSKA

	// MaxSKAAmount is the maximum SKA amount as an Amount type.
	MaxSKAAmount = Amount(MaxSKAAtoms)
)

// String returns the string representation of the coin type.
func (ct CoinType) String() string {
	switch {
	case ct == CoinTypeVAR:
		return "VAR"
	case ct == CoinTypeSKA:
		return "SKA" // Backward compatibility: first SKA coin shows as "SKA"
	case ct > CoinTypeSKA && ct <= CoinTypeMax:
		return fmt.Sprintf("SKA-%d", uint8(ct))
	default:
		return fmt.Sprintf("Unknown(%d)", uint8(ct))
	}
}

// IsValid returns whether the coin type is valid.
func (ct CoinType) IsValid() bool {
	return ct <= CoinTypeMax
}

// AtomsPerCoin returns the number of atoms per coin for the given coin type.
func (ct CoinType) AtomsPerCoin() int64 {
	switch {
	case ct == CoinTypeVAR:
		return AtomsPerVAR
	case ct >= CoinTypeSKA && ct <= CoinTypeMax:
		return AtomsPerSKA // All SKA variants use same precision
	default:
		return 0
	}
}

// MaxAtoms returns the maximum number of atoms for the given coin type.
func (ct CoinType) MaxAtoms() int64 {
	switch {
	case ct == CoinTypeVAR:
		return MaxVARAtoms
	case ct >= CoinTypeSKA && ct <= CoinTypeMax:
		return MaxSKAAtoms // All SKA variants have same max supply
	default:
		return 0
	}
}

// MaxAmount returns the maximum amount for the given coin type.
func (ct CoinType) MaxAmount() Amount {
	switch {
	case ct == CoinTypeVAR:
		return MaxVARAmount
	case ct >= CoinTypeSKA && ct <= CoinTypeMax:
		return MaxSKAAmount // All SKA variants have same max supply
	default:
		return 0
	}
}

// ParseCoinType parses a string representation of a coin type.
func ParseCoinType(s string) (CoinType, error) {
	switch s {
	case "VAR", "var":
		return CoinTypeVAR, nil
	case "SKA", "ska":
		return CoinTypeSKA, nil
	default:
		return 0, fmt.Errorf("unknown coin type: %s", s)
	}
}

// ValidateCoinType validates that a coin type value is within valid range.
func ValidateCoinType(ct CoinType) error {
	if !ct.IsValid() {
		return fmt.Errorf("invalid coin type: %d", uint8(ct))
	}
	return nil
}

// ErrInvalidCoinType is returned when an invalid coin type is encountered.
var ErrInvalidCoinType = errors.New("invalid coin type")

// ErrCoinTypeMismatch is returned when coin types don't match in operations.
var ErrCoinTypeMismatch = errors.New("coin type mismatch")
