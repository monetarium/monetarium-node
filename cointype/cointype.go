// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package cointype

import (
	"errors"
	"fmt"
)

// CoinType represents the type of native coin in the Monetarium network.
// The network supports VAR (Varta) and multiple SKA (Skarb) coin types.
type CoinType uint8

const (
	// CoinTypeVAR represents Varta coins - the original mined cryptocurrency
	// that functions as network shares. VAR holders earn transaction fees
	// and can purchase PoS tickets. VAR is always active.
	CoinTypeVAR CoinType = 0

	// CoinTypeMax defines the maximum valid coin type value.
	// SKA coin types range from 1-255 and are activated dynamically.
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

// NOTE: SKA coin amounts use big.Int via the SKAAmount type.
// MaxSupply and AtomsPerCoin for each SKA type are configured in SKACoinConfig.
// See AtomsPerSKACoin in ska_amount.go for the default (1e18).

// Amount represents a coin amount in atoms.
type Amount int64

// IsVAR returns true if this is the VAR coin type (0).
// VAR has special properties: it's mined, used for staking, and always active.
func (ct CoinType) IsVAR() bool {
	return ct == CoinTypeVAR
}

// IsSKA returns true if this is a SKA coin type (1-255).
// SKA coins are asset-backed tokens that are activated dynamically.
func (ct CoinType) IsSKA() bool {
	return ct >= 1 && ct <= CoinTypeMax
}

// IsValid returns whether the coin type is within valid range (0-255).
func (ct CoinType) IsValid() bool {
	return ct <= CoinTypeMax
}

// String returns the string representation of the coin type.
func (ct CoinType) String() string {
	switch {
	case ct == CoinTypeVAR:
		return "VAR"
	case ct >= 1 && ct <= CoinTypeMax:
		return fmt.Sprintf("SKA-%d", uint8(ct))
	default:
		return fmt.Sprintf("Unknown(%d)", uint8(ct))
	}
}

// AtomsPerCoin returns the number of atoms per coin for VAR.
// For SKA coins, use AtomsPerSKACoin (big.Int) from ska_amount.go,
// or get AtomsPerCoin from the SKACoinConfig for per-coin-type configuration.
func (ct CoinType) AtomsPerCoin() int64 {
	if ct == CoinTypeVAR {
		return AtomsPerVAR
	}
	// SKA uses big.Int - this method only works for VAR
	return 0
}

// UsesBigInt returns true if this coin type requires big.Int for amounts.
// SKA coins use big.Int due to their 18 decimal precision and large supply.
func (ct CoinType) UsesBigInt() bool {
	return ct.IsSKA()
}

// MaxAtoms returns the maximum number of atoms for VAR.
// For SKA coins, get MaxSupply from the SKACoinConfig (it's a *big.Int).
func (ct CoinType) MaxAtoms() int64 {
	if ct == CoinTypeVAR {
		return MaxVARAtoms
	}
	// SKA max supply is per-config and uses big.Int
	return 0
}

// MaxAmount returns the maximum amount for VAR.
// For SKA coins, get MaxSupply from the SKACoinConfig (it's a *big.Int).
func (ct CoinType) MaxAmount() Amount {
	if ct == CoinTypeVAR {
		return MaxVARAmount
	}
	// SKA max supply is per-config and uses big.Int
	return 0
}

// ParseCoinType parses a string representation of a coin type.
// Only supports parsing "VAR" and "SKA" - specific SKA types use numeric parsing.
func ParseCoinType(s string) (CoinType, error) {
	switch s {
	case "VAR", "var":
		return CoinTypeVAR, nil
	case "SKA", "ska":
		return CoinType(1), nil // Default to SKA-1 for backward compatibility
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

// ErrInactiveCoinType is returned when attempting to use an inactive coin type.
var ErrInactiveCoinType = errors.New("inactive coin type")

// ChainParams interface defines the methods needed from chaincfg.Params
// for coin type activation checking. This avoids import cycles.
type ChainParams interface {
	GetSKACoinConfig(CoinType) SKACoinConfig
}

// SKACoinConfig represents the configuration for a SKA coin type.
// This mirrors the structure in chaincfg to avoid import cycles.
type SKACoinConfig interface {
	IsActive() bool
}

// IsActive returns true if the coin type is active on the given chain.
// VAR is always active. SKA coins are active based on chain configuration.
func (ct CoinType) IsActive(params ChainParams) bool {
	if ct.IsVAR() {
		return true // VAR is always active
	}

	if ct.IsSKA() {
		config := params.GetSKACoinConfig(ct)
		return config != nil && config.IsActive()
	}

	return false // Invalid coin types are never active
}
