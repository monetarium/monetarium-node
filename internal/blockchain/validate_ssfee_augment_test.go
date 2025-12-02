// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/decred/dcrd/blockchain/stake/v5"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
)

// TestCheckSSFeeNullInput verifies that null-input SSFee transactions
// are accepted (baseline behavior).
func TestCheckSSFeeNullInput(t *testing.T) {
	// Create a basic null-input SSFee transaction
	tx := wire.NewMsgTx()
	tx.Version = 3

	// Null input
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex, wire.TxTreeRegular),
		Sequence: wire.MaxTxInSequenceNum,
		ValueIn:  1000,
	})

	// Output
	pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
	pkScript = append(pkScript, make([]byte, 20)...)
	pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		Version:  0,
		PkScript: pkScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// OP_RETURN marker (SF)
	opReturnScript := []byte{
		txscript.OP_RETURN,
		txscript.OP_DATA_6,
		'S', 'F', // Stake Fee marker
		0, 0, 0, 0, // height placeholder
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    0,
		Version:  0,
		PkScript: opReturnScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Validate
	if err := stake.CheckSSFee(tx); err != nil {
		t.Errorf("CheckSSFee rejected null-input SSFee: %v", err)
	}
}

// TestCheckSSFeeAugmentedInput verifies that augmented SSFee transactions
// (with real UTXO inputs) are accepted.
func TestCheckSSFeeAugmentedInput(t *testing.T) {
	// Create an augmented SSFee transaction with real input
	tx := wire.NewMsgTx()
	tx.Version = 3

	// Real UTXO input (not null)
	prevHash, _ := chainhash.NewHashFromStr("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *prevHash,
			Index: 0,
			Tree:  wire.TxTreeStake,
		},
		Sequence: wire.MaxTxInSequenceNum,
		ValueIn:  5000, // Existing UTXO value
	})

	// Output (existing value + fee)
	pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
	pkScript = append(pkScript, make([]byte, 20)...)
	pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tx.AddTxOut(&wire.TxOut{
		Value:    6000, // 5000 (input) + 1000 (fee)
		Version:  0,
		PkScript: pkScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// OP_RETURN marker (SF)
	opReturnScript := []byte{
		txscript.OP_RETURN,
		txscript.OP_DATA_6,
		'S', 'F', // Stake Fee marker
		0, 0, 0, 0, // height placeholder
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    0,
		Version:  0,
		PkScript: opReturnScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Validate - should accept augmented SSFee
	if err := stake.CheckSSFee(tx); err != nil {
		t.Errorf("CheckSSFee rejected augmented SSFee: %v", err)
	}
}

// TestCheckSSFeeMultipleInputs verifies that SSFee with multiple inputs
// is rejected (must have exactly 1 input).
func TestCheckSSFeeMultipleInputs(t *testing.T) {
	tx := wire.NewMsgTx()
	tx.Version = 3

	// Two inputs (invalid)
	prevHash, _ := chainhash.NewHashFromStr("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *prevHash, Index: 0, Tree: wire.TxTreeStake},
		Sequence:         wire.MaxTxInSequenceNum,
		ValueIn:          5000,
	})
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *prevHash, Index: 1, Tree: wire.TxTreeStake},
		Sequence:         wire.MaxTxInSequenceNum,
		ValueIn:          3000,
	})

	// Output
	pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
	pkScript = append(pkScript, make([]byte, 20)...)
	pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tx.AddTxOut(&wire.TxOut{
		Value:    8000,
		Version:  0,
		PkScript: pkScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// OP_RETURN marker
	opReturnScript := []byte{
		txscript.OP_RETURN,
		txscript.OP_DATA_6,
		'S', 'F',
		0, 0, 0, 0,
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    0,
		Version:  0,
		PkScript: opReturnScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Validate - should reject multiple inputs
	if err := stake.CheckSSFee(tx); err == nil {
		t.Error("CheckSSFee should reject SSFee with multiple inputs")
	}
}

// TestCheckSSFeeMissingMarker verifies that SSFee without SF/MF marker
// is rejected.
func TestCheckSSFeeMissingMarker(t *testing.T) {
	tx := wire.NewMsgTx()
	tx.Version = 3

	// Null input
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex, wire.TxTreeRegular),
		Sequence: wire.MaxTxInSequenceNum,
		ValueIn:  1000,
	})

	// Output without OP_RETURN marker
	pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
	pkScript = append(pkScript, make([]byte, 20)...)
	pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		Version:  0,
		PkScript: pkScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Validate - should reject (missing marker)
	if err := stake.CheckSSFee(tx); err == nil {
		t.Error("CheckSSFee should reject SSFee without SF/MF marker")
	}
}

// TestCheckSSFeeMixedCoinTypes verifies that SSFee with mixed coin types
// in outputs is rejected.
func TestCheckSSFeeMixedCoinTypes(t *testing.T) {
	tx := wire.NewMsgTx()
	tx.Version = 3

	// Null input
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex, wire.TxTreeRegular),
		Sequence: wire.MaxTxInSequenceNum,
		ValueIn:  2000,
	})

	// Output 1: VAR
	pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
	pkScript = append(pkScript, make([]byte, 20)...)
	pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		Version:  0,
		PkScript: pkScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Output 2: SKA-1 (mixed - invalid)
	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		Version:  0,
		PkScript: pkScript,
		CoinType: 1, // SKA-1
	})

	// OP_RETURN marker
	opReturnScript := []byte{
		txscript.OP_RETURN,
		txscript.OP_DATA_6,
		'S', 'F',
		0, 0, 0, 0,
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    0,
		Version:  0,
		PkScript: opReturnScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Validate - should reject mixed coin types
	if err := stake.CheckSSFee(tx); err == nil {
		t.Error("CheckSSFee should reject SSFee with mixed coin types")
	}
}

// TestCheckSSFeeMinerVARRejection verifies that miner SSFee (MF marker)
// cannot use VAR coin type (VAR miner fees go to coinbase).
func TestCheckSSFeeMinerVARRejection(t *testing.T) {
	tx := wire.NewMsgTx()
	tx.Version = 3

	// Null input
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
			wire.MaxPrevOutIndex, wire.TxTreeRegular),
		Sequence: wire.MaxTxInSequenceNum,
		ValueIn:  1000,
	})

	// Output with VAR
	pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
	pkScript = append(pkScript, make([]byte, 20)...)
	pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)

	tx.AddTxOut(&wire.TxOut{
		Value:    1000,
		Version:  0,
		PkScript: pkScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// OP_RETURN marker with MF (Miner Fee) - invalid for VAR
	opReturnScript := []byte{
		txscript.OP_RETURN,
		txscript.OP_DATA_6,
		'M', 'F', // Miner Fee marker
		0, 0, 0, 0,
	}
	tx.AddTxOut(&wire.TxOut{
		Value:    0,
		Version:  0,
		PkScript: opReturnScript,
		CoinType: cointype.CoinTypeVAR,
	})

	// Validate - should reject (miner SSFee cannot use VAR)
	if err := stake.CheckSSFee(tx); err == nil {
		t.Error("CheckSSFee should reject miner SSFee (MF) with VAR coin type")
	}
}

// TestDetermineSSFeeType verifies that SSFee transactions are correctly
// identified as TxTypeSSFee regardless of input type (null or real).
func TestDetermineSSFeeType(t *testing.T) {
	tests := []struct {
		name          string
		createTx      func() *wire.MsgTx
		shouldBeSSFee bool
	}{
		{
			name: "Null input SSFee",
			createTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx()
				tx.Version = 3
				tx.AddTxIn(&wire.TxIn{
					PreviousOutPoint: *wire.NewOutPoint(&chainhash.Hash{},
						wire.MaxPrevOutIndex, wire.TxTreeRegular),
					Sequence: wire.MaxTxInSequenceNum,
					ValueIn:  1000,
				})
				pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
				pkScript = append(pkScript, make([]byte, 20)...)
				pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)
				tx.AddTxOut(&wire.TxOut{
					Value:    1000,
					Version:  0,
					PkScript: pkScript,
					CoinType: cointype.CoinTypeVAR,
				})
				opReturnScript := []byte{
					txscript.OP_RETURN,
					txscript.OP_DATA_6,
					'S', 'F',
					0, 0, 0, 0,
				}
				tx.AddTxOut(&wire.TxOut{
					Value:    0,
					Version:  0,
					PkScript: opReturnScript,
					CoinType: cointype.CoinTypeVAR,
				})
				return tx
			},
			shouldBeSSFee: true,
		},
		{
			name: "Augmented SSFee",
			createTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx()
				tx.Version = 3
				prevHash, _ := chainhash.NewHashFromStr("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
				tx.AddTxIn(&wire.TxIn{
					PreviousOutPoint: wire.OutPoint{
						Hash:  *prevHash,
						Index: 0,
						Tree:  wire.TxTreeStake,
					},
					Sequence: wire.MaxTxInSequenceNum,
					ValueIn:  5000,
				})
				pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
				pkScript = append(pkScript, make([]byte, 20)...)
				pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)
				tx.AddTxOut(&wire.TxOut{
					Value:    6000,
					Version:  0,
					PkScript: pkScript,
					CoinType: cointype.CoinTypeVAR,
				})
				opReturnScript := []byte{
					txscript.OP_RETURN,
					txscript.OP_DATA_6,
					'S', 'F',
					0, 0, 0, 0,
				}
				tx.AddTxOut(&wire.TxOut{
					Value:    0,
					Version:  0,
					PkScript: opReturnScript,
					CoinType: cointype.CoinTypeVAR,
				})
				return tx
			},
			shouldBeSSFee: true,
		},
		{
			name: "Regular transaction",
			createTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx()
				tx.Version = 1
				prevHash, _ := chainhash.NewHashFromStr("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
				tx.AddTxIn(&wire.TxIn{
					PreviousOutPoint: wire.OutPoint{
						Hash:  *prevHash,
						Index: 0,
						Tree:  wire.TxTreeRegular,
					},
					Sequence: wire.MaxTxInSequenceNum,
					ValueIn:  10000,
				})
				pkScript := []byte{txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20}
				pkScript = append(pkScript, make([]byte, 20)...)
				pkScript = append(pkScript, txscript.OP_EQUALVERIFY, txscript.OP_CHECKSIG)
				tx.AddTxOut(&wire.TxOut{
					Value:    9000,
					Version:  0,
					PkScript: pkScript,
					CoinType: cointype.CoinTypeVAR,
				})
				return tx
			},
			shouldBeSSFee: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := test.createTx()
			txType := stake.DetermineTxType(tx)
			isSSFee := txType == stake.TxTypeSSFee

			if isSSFee != test.shouldBeSSFee {
				t.Errorf("DetermineTxType: expected SSFee=%v, got %v (type=%v)",
					test.shouldBeSSFee, isSSFee, txType)
			}
		})
	}
}
