// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
	"github.com/monetarium/monetarium-node/txscript"
	"github.com/monetarium/monetarium-node/txscript/stdaddr"
	"github.com/monetarium/monetarium-node/wire"
)

// TestSSFeeAugmentation_VAR_NullInput tests VAR staker SSFee null-input creation
// (no previous SSFee UTXO exists)
func TestSSFeeAugmentation_VAR_NullInput(t *testing.T) {
	// Create mock voter with VAR reward
	voter := createMockVoterWithConsolidationAddr(t, cointype.CoinTypeVAR, 1000, makeTestHash160(0xAA))
	voters := []*dcrutil.Tx{voter}

	// Create SSFee without SSFeeIndex (null-input mode)
	ssFeeTxns, err := createSSFeeTxBatched(cointype.CoinTypeVAR, 1000, voters, 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create VAR SSFee: %v", err)
	}

	if len(ssFeeTxns) != 1 {
		t.Fatalf("Expected 1 SSFee transaction, got %d", len(ssFeeTxns))
	}

	tx := ssFeeTxns[0].MsgTx()

	// Verify null input
	if len(tx.TxIn) != 1 {
		t.Fatalf("Expected 1 input, got %d", len(tx.TxIn))
	}
	if tx.TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
		t.Errorf("Expected null input (MaxPrevOutIndex), got %d", tx.TxIn[0].PreviousOutPoint.Index)
	}
	if tx.TxIn[0].BlockHeight != wire.NullBlockHeight {
		t.Errorf("Expected null fraud proof (NullBlockHeight), got %d", tx.TxIn[0].BlockHeight)
	}

	// Verify output value equals fee
	if len(tx.TxOut) < 2 {
		t.Fatalf("Expected at least 2 outputs, got %d", len(tx.TxOut))
	}
	// Output[0] = OP_RETURN, Output[1] = payment
	if tx.TxOut[1].Value != 1000 {
		t.Errorf("Expected output value 1000, got %d", tx.TxOut[1].Value)
	}
	if tx.TxOut[1].CoinType != cointype.CoinTypeVAR {
		t.Errorf("Expected VAR coin type, got %d", tx.TxOut[1].CoinType)
	}
}

// TestSSFeeAugmentation_VAR_Augmented documents VAR staker SSFee augmentation
// NOTE: This requires SSFeeIndex integration which needs full blockchain harness
func TestSSFeeAugmentation_VAR_Augmented(t *testing.T) {
	t.Skip("Requires SSFeeIndex integration - tested in blockchain integration tests")
	// When SSFeeIndex finds an existing VAR staker SSFee UTXO:
	// - Input: Real outpoint referencing previous SSFee UTXO
	// - Fraud proof: BlockHeight and BlockIndex from SSFeeIndex
	// - Output value: Previous UTXO value + new fee
	// - Consolidation: Single growing UTXO instead of multiple dust UTXOs
}

// TestSSFeeAugmentation_SKA_Staker_NullInput tests SKA staker SSFee null-input creation
func TestSSFeeAugmentation_SKA_Staker_NullInput(t *testing.T) {
	testCases := []struct {
		name     string
		coinType cointype.CoinType
		fee      int64
	}{
		{
			name:     "SKA-1 staker null-input",
			coinType: cointype.CoinType(1),
			fee:      1500,
		},
		{
			name:     "SKA-2 staker null-input",
			coinType: cointype.CoinType(2),
			fee:      2500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			voter := createMockVoterWithConsolidationAddr(t, tc.coinType, tc.fee, makeTestHash160(0xBB))
			voters := []*dcrutil.Tx{voter}

			ssFeeTxns, err := createSSFeeTxBatched(tc.coinType, tc.fee, voters, 200, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("Failed to create SKA staker SSFee: %v", err)
			}

			if len(ssFeeTxns) != 1 {
				t.Fatalf("Expected 1 SSFee transaction, got %d", len(ssFeeTxns))
			}

			tx := ssFeeTxns[0].MsgTx()

			// Verify null input
			if tx.TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
				t.Errorf("Expected null input")
			}

			// Verify output
			if tx.TxOut[1].Value != tc.fee {
				t.Errorf("Expected output %d, got %d", tc.fee, tx.TxOut[1].Value)
			}
			if tx.TxOut[1].CoinType != tc.coinType {
				t.Errorf("Expected coin type %d, got %d", tc.coinType, tx.TxOut[1].CoinType)
			}
		})
	}
}

// TestSSFeeAugmentation_SKA_Staker_Augmented documents SKA staker SSFee augmentation
func TestSSFeeAugmentation_SKA_Staker_Augmented(t *testing.T) {
	t.Skip("Requires SSFeeIndex integration - tested in blockchain integration tests")
	// When SSFeeIndex finds an existing SKA staker SSFee UTXO:
	// - Input: Real outpoint with fraud proof (BlockHeight, BlockIndex)
	// - Output value: Previous value + new fee (e.g., 5000 + 1500 = 6500)
	// - Security: Maturity exemption allows augmented SSFee to spend immature SSFee
	//   because output also requires maturity before external spending
	// - Consolidation: One UTXO per (coinType, consolidation address) instead of dust
}

// TestSSFeeAugmentation_SKA_Miner_NullInput tests SKA miner SSFee null-input creation
func TestSSFeeAugmentation_SKA_Miner_NullInput(t *testing.T) {
	testCases := []struct {
		name     string
		coinType cointype.CoinType
		fee      int64
	}{
		{
			name:     "SKA-1 miner null-input",
			coinType: cointype.CoinType(1),
			fee:      3000,
		},
		{
			name:     "SKA-2 miner null-input",
			coinType: cointype.CoinType(2),
			fee:      4000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			minerAddr := createMockAddress(t)

			minerSSFeeTx, err := createMinerSSFeeTx(tc.coinType, tc.fee, minerAddr, 250, nil, nil, nil, nil)
			if err != nil {
				t.Fatalf("Failed to create SKA miner SSFee: %v", err)
			}

			tx := minerSSFeeTx.MsgTx()

			// Verify null input
			if len(tx.TxIn) != 1 {
				t.Fatalf("Expected 1 input, got %d", len(tx.TxIn))
			}
			if tx.TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
				t.Errorf("Expected null input")
			}
			if tx.TxIn[0].BlockHeight != wire.NullBlockHeight {
				t.Errorf("Expected null fraud proof")
			}

			// Verify output
			if len(tx.TxOut) < 2 {
				t.Fatalf("Expected at least 2 outputs, got %d", len(tx.TxOut))
			}
			if tx.TxOut[1].Value != tc.fee {
				t.Errorf("Expected output %d, got %d", tc.fee, tx.TxOut[1].Value)
			}
			if tx.TxOut[1].CoinType != tc.coinType {
				t.Errorf("Expected coin type %d, got %d", tc.coinType, tx.TxOut[1].CoinType)
			}

			// Verify OP_SSGEN-tagged output
			if len(tx.TxOut[1].PkScript) < 1 {
				t.Fatal("Missing payment script")
			}
			if tx.TxOut[1].PkScript[0] != txscript.OP_SSGEN {
				t.Errorf("Expected OP_SSGEN tag, got %x", tx.TxOut[1].PkScript[0])
			}
		})
	}
}

// TestSSFeeAugmentation_SKA_Miner_Augmented documents SKA miner SSFee augmentation
func TestSSFeeAugmentation_SKA_Miner_Augmented(t *testing.T) {
	t.Skip("Requires SSFeeIndex integration - tested in blockchain integration tests")
	// When SSFeeIndex finds an existing SKA miner SSFee UTXO:
	// - SSFeeIndex lookup: Uses miner address hash160 extracted from payScript
	// - Input: Real outpoint with fraud proof (BlockHeight, BlockIndex)
	// - Output value: Previous value + new fee (e.g., 8000 + 2000 = 10000)
	// - Architecture: Same as staker SSFee - uses SSFeeIndex for persistent tracking
	// - Consolidation: One UTXO per (coinType, miner address) instead of dust
}

// TestSSFeeAugmentation_DoubleSpendPrevention tests that spent UTXOs are not used
func TestSSFeeAugmentation_DoubleSpendPrevention(t *testing.T) {
	t.Skip("Requires UtxoViewpoint integration - tested in blockchain integration tests")
	// This tests the double-spend prevention fix:
	// 1. User spends mature SSFee UTXO → transaction goes to mempool
	// 2. Block template generation runs
	// 3. SSFeeIndex.LookupUTXO() finds the UTXO (still unspent in blockchain view)
	// 4. blockUtxos.LookupEntry() checks if UTXO is spent in mempool
	// 5. If spent: Falls back to null-input SSFee (creates new UTXO)
	// 6. No double-spend conflict occurs
	// 7. User transaction is prioritized over augmentation (prevents deadlock)
}

// TestSSFeeConsolidatedUTXOSpendability tests consolidated SSFee UTXO maturity
func TestSSFeeConsolidatedUTXOSpendability(t *testing.T) {
	t.Skip("Requires blockchain harness - tested in blockchain integration tests")
	// This would test that consolidated SSFee UTXOs:
	// 1. Require 16 blocks maturity before external spending
	// 2. Can be augmented by SSFee before maturity (maturity exemption)
	// 3. Can be spent by users after maturity
	// 4. Validation enforced in blockchain/validate.go:CheckTransactionInputs
}

// TestSSFeeAugmentation_MultipleRounds tests accumulation across blocks
func TestSSFeeAugmentation_MultipleRounds(t *testing.T) {
	// This test demonstrates the difference between null-input and augmented SSFee
	voter := createMockVoterWithConsolidationAddr(t, cointype.CoinType(1), 1000, makeTestHash160(0xCC))
	voters := []*dcrutil.Tx{voter}

	// Round 1: Create initial SSFee (null input)
	round1Txns, err := createSSFeeTxBatched(cointype.CoinType(1), 1000, voters, 100, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Round 1 failed: %v", err)
	}

	// Round 2: Create another SSFee (also null input without SSFeeIndex)
	round2Txns, err := createSSFeeTxBatched(cointype.CoinType(1), 1000, voters, 101, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Round 2 failed: %v", err)
	}

	// Without augmentation: 2 separate UTXOs (dust accumulation)
	totalUTXOs := len(round1Txns) + len(round2Txns)
	if totalUTXOs != 2 {
		t.Errorf("Expected 2 UTXOs without augmentation, got %d", totalUTXOs)
	}

	t.Logf("Without SSFeeIndex: %d UTXOs created (dust accumulation)", totalUTXOs)
	t.Logf("With SSFeeIndex: 1 UTXO (consolidated: 1000 → 2000)")

	// Verify both are null-input transactions
	for i, txns := range [][]*dcrutil.Tx{round1Txns, round2Txns} {
		for j, tx := range txns {
			if tx.MsgTx().TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
				t.Errorf("Round %d, tx %d: Expected null input", i+1, j)
			}
		}
	}
}

// Helper functions

func createMockVoterWithConsolidationAddr(t *testing.T, coinType cointype.CoinType, reward int64, hash160 []byte) *dcrutil.Tx {
	t.Helper()
	voteTx := wire.NewMsgTx()
	voteTx.Version = 3

	// Vote structure:
	// output[0] = block reference (OP_RETURN + 40 bytes)
	// output[1] = vote bits (OP_RETURN)
	// output[2] = VAR reward
	// output[3] = consolidation address (OP_RETURN + "SC" + hash160)

	// Block reference output (simplified)
	voteTx.AddTxOut(&wire.TxOut{
		Value:    0,
		CoinType: cointype.CoinTypeVAR,
		PkScript: append([]byte{txscript.OP_RETURN, 0x28}, make([]byte, 40)...), // OP_RETURN + OP_DATA_40 + 40 bytes
	})

	// Vote bits output
	voteTx.AddTxOut(&wire.TxOut{
		Value:    0,
		CoinType: cointype.CoinTypeVAR,
		PkScript: []byte{txscript.OP_RETURN, 0x02, 0x00, 0x00}, // OP_RETURN + OP_DATA_2 + votebits
	})

	// Reward output with P2PKH script
	rewardPkScript := make([]byte, 0, 25)
	rewardPkScript = append(rewardPkScript, txscript.OP_DUP)
	rewardPkScript = append(rewardPkScript, txscript.OP_HASH160)
	rewardPkScript = append(rewardPkScript, txscript.OP_DATA_20)
	rewardPkScript = append(rewardPkScript, hash160...)
	rewardPkScript = append(rewardPkScript, txscript.OP_EQUALVERIFY)
	rewardPkScript = append(rewardPkScript, txscript.OP_CHECKSIG)

	voteTx.AddTxOut(&wire.TxOut{
		Value:    reward,
		CoinType: coinType,
		Version:  0,
		PkScript: rewardPkScript,
	})

	// Consolidation address output: OP_RETURN + OP_DATA_22 + "SC" + hash160
	consolidationScript := make([]byte, 24)
	consolidationScript[0] = txscript.OP_RETURN // 0x6a
	consolidationScript[1] = 0x16               // OP_DATA_22
	consolidationScript[2] = 0x53               // 'S'
	consolidationScript[3] = 0x43               // 'C'
	copy(consolidationScript[4:24], hash160)

	voteTx.AddTxOut(&wire.TxOut{
		Value:    0,
		CoinType: cointype.CoinTypeVAR,
		Version:  0,
		PkScript: consolidationScript,
	})

	return dcrutil.NewTx(voteTx)
}

func createMockAddress(t *testing.T) stdaddr.Address {
	t.Helper()
	hash160 := makeTestHash160(0x12)
	addr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(hash160, mockMainNetParams())
	if err != nil {
		t.Fatalf("Failed to create mock address: %v", err)
	}
	return addr
}

func makeTestHash160(seed byte) []byte {
	hash160 := make([]byte, 20)
	for i := range hash160 {
		hash160[i] = seed + byte(i)
	}
	return hash160
}

func mockMainNetParams() *chaincfg.Params {
	return &chaincfg.Params{
		PubKeyHashAddrID: [2]byte{0x13, 0x86}, // Mainnet P2PKH prefix
	}
}
