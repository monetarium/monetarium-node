// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"testing"
)

// TestNewTxOutWithCoinType tests the NewTxOutWithCoinType function.
func TestNewTxOutWithCoinType(t *testing.T) {
	pkScript := []byte{0x76, 0xa9, 0x14} // Sample script
	value := int64(100000000)            // 1 coin in atoms

	tests := []struct {
		name     string
		value    int64
		coinType CoinType
		pkScript []byte
	}{
		{"VAR TxOut", value, CoinTypeVAR, pkScript},
		{"SKA TxOut", value, CoinTypeSKA, pkScript},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			txOut := NewTxOutWithCoinType(test.value, test.coinType, test.pkScript)

			if txOut.Value != test.value {
				t.Errorf("Expected value %d, got %d", test.value, txOut.Value)
			}

			if txOut.CoinType != test.coinType {
				t.Errorf("Expected coin type %d, got %d", test.coinType, txOut.CoinType)
			}

			if !bytes.Equal(txOut.PkScript, test.pkScript) {
				t.Errorf("Expected script %x, got %x", test.pkScript, txOut.PkScript)
			}
		})
	}
}

// TestNewTxOutBackwardCompatibility tests that NewTxOut defaults to VAR.
func TestNewTxOutBackwardCompatibility(t *testing.T) {
	pkScript := []byte{0x76, 0xa9, 0x14}
	value := int64(100000000)

	txOut := NewTxOut(value, pkScript)

	if txOut.CoinType != CoinTypeVAR {
		t.Errorf("Expected NewTxOut to default to VAR, got %d", txOut.CoinType)
	}
}

// TestTxOutSerialization tests that TxOut serialization includes coin type.
func TestTxOutSerialization(t *testing.T) {
	pkScript := []byte{0x76, 0xa9, 0x14}
	value := int64(100000000)

	tests := []struct {
		name     string
		coinType CoinType
	}{
		{"VAR serialization", CoinTypeVAR},
		{"SKA serialization", CoinTypeSKA},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			txOut := NewTxOutWithCoinType(value, test.coinType, pkScript)

			// Test serialization
			var buf bytes.Buffer
			err := writeTxOut(&buf, ProtocolVersion, TxVersion, txOut)
			if err != nil {
				t.Errorf("Serialization failed: %v", err)
			}

			// Test deserialization
			var deserializedTxOut TxOut
			reader := bytes.NewReader(buf.Bytes())
			err = readTxOut(reader, ProtocolVersion, TxVersion, &deserializedTxOut)
			if err != nil {
				t.Errorf("Deserialization failed: %v", err)
			}

			// Verify round-trip
			if deserializedTxOut.Value != txOut.Value {
				t.Errorf("Value mismatch: expected %d, got %d", txOut.Value, deserializedTxOut.Value)
			}

			if deserializedTxOut.CoinType != txOut.CoinType {
				t.Errorf("CoinType mismatch: expected %d, got %d", txOut.CoinType, deserializedTxOut.CoinType)
			}

			if deserializedTxOut.Version != txOut.Version {
				t.Errorf("Version mismatch: expected %d, got %d", txOut.Version, deserializedTxOut.Version)
			}

			if !bytes.Equal(deserializedTxOut.PkScript, txOut.PkScript) {
				t.Errorf("Script mismatch: expected %x, got %x", txOut.PkScript, deserializedTxOut.PkScript)
			}
		})
	}
}

// TestTxOutSerializeSize tests that SerializeSize accounts for coin type.
func TestTxOutSerializeSize(t *testing.T) {
	pkScript := []byte{0x76, 0xa9, 0x14, 0x01, 0x02} // 5 bytes
	value := int64(100000000)

	txOut := NewTxOutWithCoinType(value, CoinTypeVAR, pkScript)

	// Expected size: 8 (value) + 1 (cointype) + 2 (version) + 1 (varint len) + 5 (script) = 17
	expectedSize := 8 + 1 + 2 + 1 + len(pkScript)
	actualSize := txOut.SerializeSize()

	if actualSize != expectedSize {
		t.Errorf("Expected serialize size %d, got %d", expectedSize, actualSize)
	}
}

// TestCoinTypeConstants tests the coin type constants.
func TestCoinTypeConstants(t *testing.T) {
	if CoinTypeVAR != 0 {
		t.Errorf("Expected CoinTypeVAR to be 0, got %d", CoinTypeVAR)
	}

	if CoinTypeSKA != 1 {
		t.Errorf("Expected CoinTypeSKA to be 1, got %d", CoinTypeSKA)
	}
}
