// Copyright (c) 2025 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/monetarium/monetarium-node/blockchain/standalone"
	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
	"github.com/monetarium/monetarium-node/wire"
)

// TestSKACrossTypeSubsidizationPrevention ensures that SKA coins of different types
// cannot subsidize each other. For example, SKA-7 inputs cannot be used to pay for
// SKA-13 outputs.
func TestSKACrossTypeSubsidizationPrevention(t *testing.T) {
	// Create a test chain configuration with SKA types active
	params := chaincfg.SimNetParams()
	// Activate SKA-2 for this test (SKA-1 is already active by default)
	if params.SKACoins[cointype.CoinType(2)] != nil {
		params.SKACoins[cointype.CoinType(2)].Active = true
	}

	// Create a test transaction with inputs of SKA-1 and outputs of SKA-2
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{1},
					Index: 0,
				},
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    50000,             // Output for SKA-2 (different from input)
				SKAValue: big.NewInt(50000), // SKA uses big.Int for amounts
				CoinType: cointype.CoinType(2),
				PkScript: []byte{0x00}, // Dummy script
			},
		},
	}

	// Create a utxo view with SKA-1 input
	view := NewUtxoViewpoint(nil)
	prevOut := wire.OutPoint{Hash: chainhash.Hash{1}, Index: 0}

	// Create a UTXO entry with coin type SKA-1 and value 100000
	entry := &UtxoEntry{
		amount:        100000,               // SKA-1 input value (legacy)
		skaAmount:     big.NewInt(100000),   // SKA uses big.Int for amounts
		coinType:      cointype.CoinType(1), // SKA-1 coin type
		pkScript:      []byte{0x00},
		blockHeight:   100,
		blockIndex:    0,
		scriptVersion: 0,
		packedFlags:   0,
	}
	view.Entries()[prevOut] = entry

	// Create test subsidyCache
	subsidyCache := standalone.NewSubsidyCache(params)

	// Test: Transaction should fail because SKA-1 inputs cannot pay for SKA-2 outputs
	utilTx := dcrutil.NewTx(tx)
	_, err := CheckTransactionInputs(
		subsidyCache,
		utilTx,
		101, // Current height
		view,
		false, // checkFraudProof
		params,
		nil,   // prevHeader
		false, // isTreasuryEnabled
		false, // isAutoRevocationsEnabled
		standalone.SSVOriginal,
	)

	// We expect an error because SKA-1 cannot subsidize SKA-2
	if err == nil {
		t.Fatalf("Expected error for cross-type SKA subsidization, but got none")
	}

	// The error should be a RuleError
	var ruleErr RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("Expected RuleError for cross-type subsidization, got: %T", err)
	}

	// Verify the error message mentions SKA(2) specifically
	errStr := err.Error()
	if !strings.Contains(errStr, "SKA(2)") || !strings.Contains(errStr, "insufficient") {
		t.Fatalf("Expected error to mention insufficient SKA(2) inputs, got: %s", errStr)
	}

	// Now test that same-type SKA transactions work correctly
	tx2 := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  chainhash.Hash{2},
					Index: 0,
				},
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    50000,             // Output for SKA-1
				SKAValue: big.NewInt(50000), // SKA uses big.Int for amounts
				CoinType: cointype.CoinType(1),
				PkScript: []byte{0x00},
			},
		},
	}

	// Add SKA-1 UTXO for the second test
	prevOut2 := wire.OutPoint{Hash: chainhash.Hash{2}, Index: 0}
	entry2 := &UtxoEntry{
		amount:        100000,               // SKA-1 input value (legacy)
		skaAmount:     big.NewInt(100000),   // SKA uses big.Int for amounts
		coinType:      cointype.CoinType(1), // SKA-1 coin type
		pkScript:      []byte{0x00},
		blockHeight:   100,
		blockIndex:    0,
		scriptVersion: 0,
		packedFlags:   0,
	}
	view.Entries()[prevOut2] = entry2

	// This should succeed because input and output are both SKA-1
	utilTx2 := dcrutil.NewTx(tx2)
	fee, err := CheckTransactionInputs(
		subsidyCache,
		utilTx2,
		101,
		view,
		false,
		params,
		nil,
		false,
		false,
		standalone.SSVOriginal,
	)

	if err != nil {
		t.Fatalf("Same-type SKA transaction failed unexpectedly: %v", err)
	}

	// Fee should be 50000 (100000 input - 50000 output)
	expectedFee := big.NewInt(50000)
	if fee.Cmp(expectedFee) != 0 {
		t.Fatalf("Expected fee %s for same-type SKA transaction, got %s", expectedFee, fee)
	}
}
