// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"testing"
)

// TestUtxoSerializationDualCoin tests UTXO serialization/deserialization
// with coin type support.
func TestUtxoSerializationDualCoin(t *testing.T) {
	tests := []struct {
		name  string
		entry *UtxoEntry
	}{
		{
			name: "VAR UTXO",
			entry: &UtxoEntry{
				amount:        100000000,
				pkScript:      []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				blockHeight:   12345,
				blockIndex:    2,
				scriptVersion: 0,
				coinType:      CoinTypeVAR,
				packedFlags:   0,
			},
		},
		{
			name: "SKA UTXO",
			entry: &UtxoEntry{
				amount:        50000000,
				pkScript:      []byte{0xa9, 0x14, 0x04, 0x05, 0x06, 0x87},
				blockHeight:   54321,
				blockIndex:    1,
				scriptVersion: 0,
				coinType:      CoinTypeSKA,
				packedFlags:   0,
			},
		},
		{
			name: "VAR Coinbase UTXO",
			entry: &UtxoEntry{
				amount:        500000000,
				pkScript:      []byte{0x21, 0x03, 0xaa, 0xbb, 0xcc},
				blockHeight:   100000,
				blockIndex:    0,
				scriptVersion: 0,
				coinType:      CoinTypeVAR,
				packedFlags:   encodeUtxoFlags(true, false, 0), // coinbase
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Serialize the entry
			serialized := serializeUtxoEntry(test.entry)
			if serialized == nil {
				t.Fatal("Serialization returned nil")
			}

			// Deserialize the entry
			deserialized, err := deserializeUtxoEntry(serialized, 0)
			if err != nil {
				t.Fatalf("Deserialization failed: %v", err)
			}

			// Compare all fields
			if deserialized.amount != test.entry.amount {
				t.Errorf("Amount mismatch: expected %d, got %d",
					test.entry.amount, deserialized.amount)
			}

			if deserialized.coinType != test.entry.coinType {
				t.Errorf("CoinType mismatch: expected %d, got %d",
					test.entry.coinType, deserialized.coinType)
			}

			if deserialized.blockHeight != test.entry.blockHeight {
				t.Errorf("BlockHeight mismatch: expected %d, got %d",
					test.entry.blockHeight, deserialized.blockHeight)
			}

			if deserialized.blockIndex != test.entry.blockIndex {
				t.Errorf("BlockIndex mismatch: expected %d, got %d",
					test.entry.blockIndex, deserialized.blockIndex)
			}

			if deserialized.scriptVersion != test.entry.scriptVersion {
				t.Errorf("ScriptVersion mismatch: expected %d, got %d",
					test.entry.scriptVersion, deserialized.scriptVersion)
			}

			if !bytes.Equal(deserialized.pkScript, test.entry.pkScript) {
				t.Errorf("PkScript mismatch: expected %x, got %x",
					test.entry.pkScript, deserialized.pkScript)
			}

			if deserialized.packedFlags != test.entry.packedFlags {
				t.Errorf("PackedFlags mismatch: expected %d, got %d",
					test.entry.packedFlags, deserialized.packedFlags)
			}
		})
	}
}

// TestUtxoSerializationBackwardCompatibility tests that version 3 entries
// can still be deserialized correctly.
func TestUtxoSerializationBackwardCompatibility(t *testing.T) {
	// Create a version 3 style entry (without coin type in serialization)
	entry := &UtxoEntry{
		amount:        100000000,
		pkScript:      []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
		blockHeight:   12345,
		blockIndex:    2,
		scriptVersion: 0,
		coinType:      CoinTypeVAR, // This won't be in the serialized data
		packedFlags:   0,
	}

	// Manually create version 3 format serialization (without coin type)
	flags := encodeFlags(entry.IsCoinBase(), entry.HasExpiry(), entry.TransactionType())
	var buf bytes.Buffer

	// Version 3 format: height + index + flags + compressed txout (no coin type)
	putVLQ(append(buf.Bytes(), make([]byte, serializeSizeVLQ(uint64(entry.blockHeight)))...), uint64(entry.blockHeight))

	serialized := make([]byte,
		serializeSizeVLQ(uint64(entry.blockHeight))+
			serializeSizeVLQ(uint64(entry.blockIndex))+
			serializeSizeVLQ(uint64(flags))+
			compressedTxOutSize(uint64(entry.amount), entry.scriptVersion, entry.pkScript, true))

	offset := putVLQ(serialized, uint64(entry.blockHeight))
	offset += putVLQ(serialized[offset:], uint64(entry.blockIndex))
	offset += putVLQ(serialized[offset:], uint64(flags))
	putCompressedTxOut(serialized[offset:], uint64(entry.amount), entry.scriptVersion, entry.pkScript, true)

	// Deserialize using the new function (should default to VAR)
	deserialized, err := deserializeUtxoEntry(serialized, 0)
	if err != nil {
		t.Fatalf("Backward compatibility deserialization failed: %v", err)
	}

	// Should default to VAR coin type
	if deserialized.coinType != CoinTypeVAR {
		t.Errorf("Expected default coin type VAR, got %d", deserialized.coinType)
	}

	// Other fields should match
	if deserialized.amount != entry.amount {
		t.Errorf("Amount mismatch: expected %d, got %d", entry.amount, deserialized.amount)
	}

	if deserialized.blockHeight != entry.blockHeight {
		t.Errorf("BlockHeight mismatch: expected %d, got %d", entry.blockHeight, deserialized.blockHeight)
	}
}

// TestUtxoSerializationSize tests that serialization size calculation is correct.
func TestUtxoSerializationSize(t *testing.T) {
	tests := []struct {
		name  string
		entry *UtxoEntry
	}{
		{
			name: "Small VAR UTXO",
			entry: &UtxoEntry{
				amount:        1000,
				pkScript:      []byte{0x76, 0xa9},
				blockHeight:   100,
				blockIndex:    1,
				scriptVersion: 0,
				coinType:      CoinTypeVAR,
				packedFlags:   0,
			},
		},
		{
			name: "Large SKA UTXO",
			entry: &UtxoEntry{
				amount:        100000000,
				pkScript:      make([]byte, 25), // Larger script
				blockHeight:   1000000,
				blockIndex:    100,
				scriptVersion: 0,
				coinType:      CoinTypeSKA,
				packedFlags:   0,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serialized := serializeUtxoEntry(test.entry)
			if serialized == nil {
				t.Fatal("Serialization returned nil")
			}

			// Calculate expected size
			flags := encodeFlags(test.entry.IsCoinBase(), test.entry.HasExpiry(), test.entry.TransactionType())
			expectedSize := serializeSizeVLQ(uint64(test.entry.blockHeight)) +
				serializeSizeVLQ(uint64(test.entry.blockIndex)) +
				serializeSizeVLQ(uint64(flags)) +
				serializeSizeVLQ(uint64(test.entry.coinType)) + // New coin type field
				compressedTxOutSize(uint64(test.entry.amount), test.entry.scriptVersion, test.entry.pkScript, true)

			if len(serialized) != expectedSize {
				t.Errorf("Serialization size mismatch: expected %d, got %d", expectedSize, len(serialized))
			}
		})
	}
}

// TestUtxoSerializationSpentEntry tests that spent entries return nil.
func TestUtxoSerializationSpentEntry(t *testing.T) {
	entry := &UtxoEntry{
		amount:        100000000,
		pkScript:      []byte{0x76, 0xa9, 0x14},
		blockHeight:   12345,
		blockIndex:    2,
		scriptVersion: 0,
		coinType:      CoinTypeVAR,
		state:         utxoStateSpent, // Mark as spent
		packedFlags:   0,
	}

	serialized := serializeUtxoEntry(entry)
	if serialized != nil {
		t.Error("Spent entry should serialize to nil")
	}
}
