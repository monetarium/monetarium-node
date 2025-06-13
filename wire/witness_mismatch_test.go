// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"errors"
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
)

// TestWitnessMismatchBug reproduces the issue where transactions serialized
// with DualCoinVersion but deserialized with older protocol versions cause
// witness count mismatches.
func TestWitnessMismatchBug(t *testing.T) {
	// Create a transaction with both VAR and SKA outputs
	tx := NewMsgTx()

	// Add a transaction input
	var zeroHash chainhash.Hash
	prevOut := NewOutPoint(&zeroHash, 0, TxTreeRegular)
	txIn := NewTxIn(prevOut, 5000000000, []byte{0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62})
	tx.AddTxIn(txIn)

	// Add VAR output
	varOutput := NewTxOutWithCoinType(100000000, CoinTypeVAR, []byte{0x76, 0xa9, 0x14})
	tx.AddTxOut(varOutput)

	// Add SKA output
	skaOutput := NewTxOutWithCoinType(50000000, CoinTypeSKA, []byte{0x76, 0xa9, 0x14})
	tx.AddTxOut(skaOutput)

	// Test serialization with DualCoinVersion
	var bufNew bytes.Buffer
	err := tx.BtcEncode(&bufNew, DualCoinVersion)
	if err != nil {
		t.Fatalf("Failed to serialize with DualCoinVersion: %v", err)
	}

	// Test deserialization with DualCoinVersion (should work)
	txNew := &MsgTx{}
	err = txNew.BtcDecode(bytes.NewReader(bufNew.Bytes()), DualCoinVersion)
	if err != nil {
		t.Fatalf("Failed to deserialize with DualCoinVersion: %v", err)
	}

	// Test deserialization with older protocol version (should fail with witness mismatch)
	txOld := &MsgTx{}
	err = txOld.BtcDecode(bytes.NewReader(bufNew.Bytes()), DualCoinVersion-1)
	if err == nil {
		t.Error("Expected error when deserializing DualCoinVersion transaction with older protocol version")
	} else {
		t.Logf("Expected error occurred: %v", err)

		// Check if it's the specific witness mismatch error
		var msgErr *MessageError
		if errors.As(err, &msgErr) {
			if msgErr.ErrorCode == ErrMismatchedWitnessCount {
				t.Logf("Confirmed: ErrMismatchedWitnessCount error reproduced")
			} else {
				t.Errorf("Got MessageError but wrong type: %v", msgErr.ErrorCode)
			}
		}
	}
}

// TestProtocolVersionCompatibility tests that transaction serialization
// is compatible across protocol versions for VAR-only transactions.
func TestProtocolVersionCompatibility(t *testing.T) {
	// Create a VAR-only transaction
	tx := NewMsgTx()

	// Add a transaction input
	var zeroHash chainhash.Hash
	prevOut := NewOutPoint(&zeroHash, 0, TxTreeRegular)
	txIn := NewTxIn(prevOut, 5000000000, []byte{0x04, 0x31, 0xdc, 0x00, 0x1b, 0x01, 0x62})
	tx.AddTxIn(txIn)

	// Add VAR output (should be compatible with older versions)
	varOutput := NewTxOut(100000000, []byte{0x76, 0xa9, 0x14})
	tx.AddTxOut(varOutput)

	// Test serialization with older protocol version
	var bufOld bytes.Buffer
	err := tx.BtcEncode(&bufOld, DualCoinVersion-1)
	if err != nil {
		t.Fatalf("Failed to serialize with older protocol version: %v", err)
	}

	// Test serialization with DualCoinVersion
	var bufNew bytes.Buffer
	err = tx.BtcEncode(&bufNew, DualCoinVersion)
	if err != nil {
		t.Fatalf("Failed to serialize with DualCoinVersion: %v", err)
	}

	// The sizes should be different due to CoinType field
	if len(bufOld.Bytes()) >= len(bufNew.Bytes()) {
		t.Errorf("Expected DualCoinVersion serialization to be larger due to CoinType field")
	}

	// Test cross-compatibility: deserialize old format with old version (should work)
	txFromOld := &MsgTx{}
	err = txFromOld.BtcDecode(bytes.NewReader(bufOld.Bytes()), DualCoinVersion-1)
	if err != nil {
		t.Errorf("Failed to deserialize old format with old protocol version: %v", err)
	} else {
		// Should default to VAR for old protocol version
		if txFromOld.TxOut[0].CoinType != CoinTypeVAR {
			t.Errorf("Expected CoinType to default to VAR, got %d", txFromOld.TxOut[0].CoinType)
		}
	}

	// Test compatibility: deserialize new format with old version (should fail)
	txFromNew := &MsgTx{}
	err = txFromNew.BtcDecode(bytes.NewReader(bufNew.Bytes()), DualCoinVersion-1)
	if err == nil {
		t.Error("Expected error when deserializing new format with old protocol version")
	}
}
