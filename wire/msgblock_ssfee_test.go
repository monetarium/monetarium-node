// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/cointype"
)

// TestBlockWithSSFeeDeserialization tests that blocks containing both
// coinbase transactions (with non-empty SignatureScript) and SSFee
// transactions (with empty SignatureScript) can be serialized and
// deserialized without corruption.
//
// This is a regression test for a bug where coinbase SignatureScript
// would become empty after deserialization when SSFee transactions
// were present in the same block.
func TestBlockWithSSFeeDeserialization(t *testing.T) {
	// Create a coinbase transaction with a 2-byte SignatureScript
	// Using OP_0 (0x00) for test simplicity
	coinbaseSigScript := []byte{0x00, 0x00}
	coinbase := NewMsgTx()
	coinbase.AddTxIn(&TxIn{
		PreviousOutPoint: *NewOutPoint(&chainhash.Hash{},
			MaxPrevOutIndex, TxTreeRegular),
		Sequence:        MaxTxInSequenceNum,
		ValueIn:         1000000,
		BlockHeight:     NullBlockHeight,
		BlockIndex:      NullBlockIndex,
		SignatureScript: coinbaseSigScript,
	})
	coinbase.AddTxOut(&TxOut{
		Value:    1000000,
		CoinType: cointype.CoinTypeVAR,
		Version:  0,
		PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03, 0x88, 0xac},
	})

	// Create an SSFee transaction with empty SignatureScript (null-input)
	ssfeeTx := NewMsgTx()
	ssfeeTx.AddTxIn(&TxIn{
		PreviousOutPoint: *NewOutPoint(&chainhash.Hash{},
			MaxPrevOutIndex, TxTreeRegular),
		Sequence:        MaxTxInSequenceNum,
		ValueIn:         5,
		BlockHeight:     NullBlockHeight,
		BlockIndex:      NullBlockIndex,
		SignatureScript: []byte{}, // Empty signature script
	})
	ssfeeTx.AddTxOut(&TxOut{
		Value:    5,
		CoinType: cointype.CoinTypeVAR,
		Version:  0,
		PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06, 0x88, 0xac},
	})

	// Create a block with both transactions
	block := &MsgBlock{
		Header: BlockHeader{
			Version:   1,
			PrevBlock: chainhash.Hash{},
			Height:    1,
		},
		Transactions:  []*MsgTx{coinbase},
		STransactions: []*MsgTx{ssfeeTx},
	}

	// Serialize the block
	var buf bytes.Buffer
	err := block.BtcEncode(&buf, ProtocolVersion)
	if err != nil {
		t.Fatalf("Failed to serialize block: %v", err)
	}

	t.Logf("Serialized block size: %d bytes", buf.Len())
	t.Logf("Coinbase SignatureScript before deserialization: %v (len=%d)",
		block.Transactions[0].TxIn[0].SignatureScript,
		len(block.Transactions[0].TxIn[0].SignatureScript))

	// Deserialize the block
	serializedData := buf.Bytes()
	deserializedBlock := &MsgBlock{}
	r := bytes.NewReader(serializedData)
	err = deserializedBlock.BtcDecode(r, ProtocolVersion)
	if err != nil {
		t.Fatalf("Failed to deserialize block: %v", err)
	}

	// Check that coinbase SignatureScript is intact
	if len(deserializedBlock.Transactions) == 0 {
		t.Fatal("Deserialized block has no transactions")
	}

	deserializedCoinbase := deserializedBlock.Transactions[0]
	if len(deserializedCoinbase.TxIn) == 0 {
		t.Fatal("Deserialized coinbase has no inputs")
	}

	deserializedSigScript := deserializedCoinbase.TxIn[0].SignatureScript
	t.Logf("Coinbase SignatureScript after deserialization: %v (len=%d)",
		deserializedSigScript, len(deserializedSigScript))

	// Verify the SignatureScript is still 2 bytes and matches original
	if len(deserializedSigScript) != len(coinbaseSigScript) {
		t.Errorf("SignatureScript length mismatch: got %d, want %d",
			len(deserializedSigScript), len(coinbaseSigScript))
	}

	if !bytes.Equal(deserializedSigScript, coinbaseSigScript) {
		t.Errorf("SignatureScript content mismatch: got %v, want %v",
			deserializedSigScript, coinbaseSigScript)
	}

	// Also verify SSFee transaction has empty SignatureScript
	if len(deserializedBlock.STransactions) == 0 {
		t.Fatal("Deserialized block has no stake transactions")
	}

	deserializedSSFee := deserializedBlock.STransactions[0]
	if len(deserializedSSFee.TxIn) == 0 {
		t.Fatal("Deserialized SSFee has no inputs")
	}

	if len(deserializedSSFee.TxIn[0].SignatureScript) != 0 {
		t.Errorf("SSFee SignatureScript should be empty, got length %d",
			len(deserializedSSFee.TxIn[0].SignatureScript))
	}
}

// TestBlockWithTxHashCalls tests that calling TxHash() on transactions
// (like during merkle root calculation) doesn't corrupt transaction data.
func TestBlockWithTxHashCalls(t *testing.T) {
	// Create a coinbase transaction
	coinbaseSigScript := []byte{0x00, 0x00}
	coinbase := NewMsgTx()
	coinbase.AddTxIn(&TxIn{
		PreviousOutPoint: *NewOutPoint(&chainhash.Hash{},
			MaxPrevOutIndex, TxTreeRegular),
		Sequence:        MaxTxInSequenceNum,
		ValueIn:         1000000,
		BlockHeight:     NullBlockHeight,
		BlockIndex:      NullBlockIndex,
		SignatureScript: coinbaseSigScript,
	})
	coinbase.AddTxOut(&TxOut{
		Value:    1000000,
		CoinType: cointype.CoinTypeVAR,
		Version:  0,
		PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03, 0x88, 0xac},
	})

	// Create multiple SSFee transactions (simulating a block with votes + SSFees)
	vote1 := NewMsgTx()
	vote1.AddTxIn(&TxIn{
		PreviousOutPoint: *NewOutPoint(&chainhash.Hash{0x01},
			0, TxTreeStake),
		Sequence:        MaxTxInSequenceNum,
		ValueIn:         100,
		BlockHeight:     NullBlockHeight,
		BlockIndex:      NullBlockIndex,
		SignatureScript: []byte{0x01, 0x02}, // Non-empty
	})
	vote1.AddTxOut(&TxOut{
		Value:    100,
		CoinType: cointype.CoinTypeVAR,
		Version:  0,
		PkScript: []byte{0x76, 0xa9, 0x14, 0x07, 0x08, 0x09, 0x88, 0xac},
	})

	ssfeeTx1 := NewMsgTx()
	ssfeeTx1.AddTxIn(&TxIn{
		PreviousOutPoint: *NewOutPoint(&chainhash.Hash{},
			MaxPrevOutIndex, TxTreeRegular),
		Sequence:        MaxTxInSequenceNum,
		ValueIn:         5,
		BlockHeight:     NullBlockHeight,
		BlockIndex:      NullBlockIndex,
		SignatureScript: []byte{}, // Empty
	})
	ssfeeTx1.AddTxOut(&TxOut{
		Value:    5,
		CoinType: cointype.CoinTypeVAR,
		Version:  0,
		PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06, 0x88, 0xac},
	})

	// CRITICAL: Call TxHash() on all transactions (simulating merkle root calculation)
	// This is what the blockchain test generator does!
	t.Log("Calling TxHash() on coinbase...")
	coinbaseHash := coinbase.TxHash()
	t.Logf("Coinbase hash: %s", coinbaseHash)
	t.Logf("Coinbase SignatureScript after TxHash: %v (len=%d)",
		coinbase.TxIn[0].SignatureScript, len(coinbase.TxIn[0].SignatureScript))

	t.Log("Calling TxHash() on vote...")
	voteHash := vote1.TxHash()
	t.Logf("Vote hash: %s", voteHash)

	t.Log("Calling TxHash() on SSFee...")
	ssfeeHash := ssfeeTx1.TxHash()
	t.Logf("SSFee hash: %s", ssfeeHash)

	t.Logf("Coinbase SignatureScript after ALL TxHash calls: %v (len=%d)",
		coinbase.TxIn[0].SignatureScript, len(coinbase.TxIn[0].SignatureScript))

	// Now create the block
	block := &MsgBlock{
		Header: BlockHeader{
			Version:   1,
			PrevBlock: chainhash.Hash{},
			Height:    11,
		},
		Transactions:  []*MsgTx{coinbase},
		STransactions: []*MsgTx{vote1, ssfeeTx1},
	}

	// Serialize the block
	var buf bytes.Buffer
	err := block.BtcEncode(&buf, ProtocolVersion)
	if err != nil {
		t.Fatalf("Failed to serialize block: %v", err)
	}

	t.Logf("Coinbase SignatureScript before deserialization: %v (len=%d)",
		block.Transactions[0].TxIn[0].SignatureScript,
		len(block.Transactions[0].TxIn[0].SignatureScript))

	// Deserialize the block
	serializedData := buf.Bytes()
	deserializedBlock := &MsgBlock{}
	r := bytes.NewReader(serializedData)
	err = deserializedBlock.BtcDecode(r, ProtocolVersion)
	if err != nil {
		t.Fatalf("Failed to deserialize block: %v", err)
	}

	// Check coinbase SignatureScript
	deserializedSigScript := deserializedBlock.Transactions[0].TxIn[0].SignatureScript
	t.Logf("Coinbase SignatureScript after deserialization: %v (len=%d)",
		deserializedSigScript, len(deserializedSigScript))

	if len(deserializedSigScript) != len(coinbaseSigScript) {
		t.Errorf("SignatureScript length mismatch: got %d, want %d",
			len(deserializedSigScript), len(coinbaseSigScript))
		t.Errorf("THIS IS THE BUG! TxHash() calls corrupted the transaction!")
	}

	if !bytes.Equal(deserializedSigScript, coinbaseSigScript) {
		t.Errorf("SignatureScript content mismatch: got %v, want %v",
			deserializedSigScript, coinbaseSigScript)
	}
}
