// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"reflect"
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
		{CoinType(99), "Unknown"},
	}

	for i, test := range tests {
		result := test.coinType.String()
		if result != test.expected {
			t.Errorf("Test %d: expected %s, got %s", i, test.expected, result)
		}
	}
}

// TestCoinTypeIsValid tests the IsValid method of CoinType.
// Note: Since CoinType is uint8 (0-255) and CoinTypeMax is 255,
// all possible uint8 values are now valid coin types.
func TestCoinTypeIsValid(t *testing.T) {
	tests := []struct {
		coinType CoinType
		expected bool
	}{
		{CoinTypeVAR, true},   // VAR coin (0)
		{CoinTypeSKA, true},   // SKA-1 coin (1)
		{CoinType(2), true},   // SKA-2 coin (2)
		{CoinType(99), true},  // SKA-99 coin (99)
		{CoinType(255), true}, // SKA-255 coin (255) - maximum
	}

	for i, test := range tests {
		result := test.coinType.IsValid()
		if result != test.expected {
			t.Errorf("Test %d: expected %t, got %t", i, test.expected, result)
		}
	}
}

// TestUtxoEntryCoinType tests the CoinType accessor method.
func TestUtxoEntryCoinType(t *testing.T) {
	tests := []struct {
		name     string
		coinType CoinType
	}{
		{"VAR entry", CoinTypeVAR},
		{"SKA entry", CoinTypeSKA},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := &UtxoEntry{
				amount:   100000000,
				coinType: test.coinType,
			}

			result := entry.CoinType()
			if result != test.coinType {
				t.Errorf("Expected coin type %d, got %d", test.coinType, result)
			}
		})
	}
}

// TestUtxoEntryAmountWithCoinType tests the AmountWithCoinType method.
func TestUtxoEntryAmountWithCoinType(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		coinType CoinType
	}{
		{"VAR 1 coin", 100000000, CoinTypeVAR},
		{"SKA 0.5 coin", 50000000, CoinTypeSKA},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := &UtxoEntry{
				amount:   test.amount,
				coinType: test.coinType,
			}

			amount, coinType := entry.AmountWithCoinType()
			if amount != test.amount {
				t.Errorf("Expected amount %d, got %d", test.amount, amount)
			}
			if coinType != test.coinType {
				t.Errorf("Expected coin type %d, got %d", test.coinType, coinType)
			}
		})
	}
}

// TestUtxoEntryClone tests that Clone includes the coinType field.
func TestUtxoEntryClone(t *testing.T) {
	original := &UtxoEntry{
		amount:        100000000,
		pkScript:      []byte{0x76, 0xa9, 0x14},
		blockHeight:   12345,
		blockIndex:    2,
		scriptVersion: 0,
		coinType:      CoinTypeSKA,
		state:         0,
		packedFlags:   0,
	}

	cloned := original.Clone()

	// Verify all fields are copied correctly
	if cloned.amount != original.amount {
		t.Errorf("Amount not cloned correctly: expected %d, got %d",
			original.amount, cloned.amount)
	}

	if cloned.coinType != original.coinType {
		t.Errorf("CoinType not cloned correctly: expected %d, got %d",
			original.coinType, cloned.coinType)
	}

	if cloned.blockHeight != original.blockHeight {
		t.Errorf("BlockHeight not cloned correctly: expected %d, got %d",
			original.blockHeight, cloned.blockHeight)
	}

	if cloned.blockIndex != original.blockIndex {
		t.Errorf("BlockIndex not cloned correctly: expected %d, got %d",
			original.blockIndex, cloned.blockIndex)
	}

	if cloned.scriptVersion != original.scriptVersion {
		t.Errorf("ScriptVersion not cloned correctly: expected %d, got %d",
			original.scriptVersion, cloned.scriptVersion)
	}

	if !reflect.DeepEqual(cloned.pkScript, original.pkScript) {
		t.Errorf("PkScript not cloned correctly: expected %x, got %x",
			original.pkScript, cloned.pkScript)
	}

	// Verify it's a deep copy (different pointers)
	if cloned == original {
		t.Error("Clone returned same pointer instead of deep copy")
	}
}

// TestUtxoEntryNilClone tests that Clone handles nil entries.
func TestUtxoEntryNilClone(t *testing.T) {
	var entry *UtxoEntry
	cloned := entry.Clone()
	if cloned != nil {
		t.Error("Clone of nil entry should return nil")
	}
}
