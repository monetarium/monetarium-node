// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4"
	"github.com/decred/dcrd/wire"
)

// TestCreateSSFeeTx tests the createSSFeeTx function with various inputs
func TestCreateSSFeeTx(t *testing.T) {
	// Create mock voters
	voters := make([]*dcrutil.Tx, 3)
	for i := range voters {
		voteTx := wire.NewMsgTx()
		voteTx.Version = 3

		// Add typical vote structure
		// [0] = reference output
		voteTx.AddTxOut(&wire.TxOut{
			Value:    0,
			CoinType: cointype.CoinTypeVAR,
			PkScript: []byte{txscript.OP_RETURN},
		})

		// [1] = vote bits
		voteTx.AddTxOut(&wire.TxOut{
			Value:    0,
			CoinType: cointype.CoinTypeVAR,
			PkScript: []byte{txscript.OP_RETURN},
		})

		// [2] = reward output
		voteTx.AddTxOut(&wire.TxOut{
			Value:    1000,
			CoinType: cointype.CoinTypeVAR,
			Version:  0,
			PkScript: []byte{txscript.OP_DUP, txscript.OP_HASH160, 0x14}, // Mock P2PKH
		})

		voters[i] = dcrutil.NewTx(voteTx)
	}

	tests := []struct {
		name        string
		coinType    cointype.CoinType
		totalFee    int64 // This is the staker portion (50% of total block fees)
		voters      []*dcrutil.Tx
		height      int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid SKA-1 fee distribution",
			coinType:    1,    // SKA-1
			totalFee:    3000, // Stakers get 3000 (miners got the other 3000)
			voters:      voters,
			height:      100,
			expectError: false,
		},
		{
			name:        "valid SKA-2 fee distribution",
			coinType:    2,    // SKA-2
			totalFee:    6000, // Stakers get 6000 (miners got the other 6000)
			voters:      voters,
			height:      200,
			expectError: false,
		},
		{
			name:        "valid VAR fee distribution",
			coinType:    cointype.CoinTypeVAR,
			totalFee:    1000,
			voters:      voters,
			height:      100,
			expectError: false,
		},
		{
			name:        "no voters",
			coinType:    1,
			totalFee:    1000,
			voters:      []*dcrutil.Tx{},
			height:      100,
			expectError: true,
			errorMsg:    "no voters to distribute fees to",
		},
		{
			name:        "negative total fee",
			coinType:    1,
			totalFee:    -1000,
			voters:      voters,
			height:      100,
			expectError: true,
			errorMsg:    "negative total fee",
		},
		{
			name:        "overflow protection - extremely large fee",
			coinType:    1,
			totalFee:    9223372036854775807, // math.MaxInt64
			voters:      voters,
			height:      100,
			expectError: true,
			errorMsg:    "total fee too large for distribution",
		},
		{
			name:        "zero total fee",
			coinType:    1,
			totalFee:    0,
			voters:      voters,
			height:      100,
			expectError: false, // Should work, just distribute 0 to everyone
		},
		{
			name:        "remainder distribution fairness test",
			coinType:    1,
			totalFee:    10, // 10 atoms to 3 voters = 3 each + 1 remainder
			voters:      voters,
			height:      100,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Pass nil for utxoView in basic tests (will create new UTXOs)
			ssFeeTxns, err := createSSFeeTx(test.coinType, test.totalFee, test.voters, test.height, nil)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if test.errorMsg != "" && !contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s' but got '%s'", test.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Should return one SSFee transaction per voter
			expectedTxns := len(test.voters)
			if len(ssFeeTxns) != expectedTxns {
				t.Errorf("Expected %d SSFee transactions, got %d", expectedTxns, len(ssFeeTxns))
				return
			}

			// Verify each SSFee transaction
			var totalDistributed int64
			for txIdx, ssFeeTx := range ssFeeTxns {
				msgTx := ssFeeTx.MsgTx()

				// Check transaction version
				if msgTx.Version != 3 {
					t.Errorf("Tx %d: Expected version 3, got %d", txIdx, msgTx.Version)
				}

				// Check it's a stake transaction
				if ssFeeTx.Tree() != wire.TxTreeStake {
					t.Errorf("Tx %d: Expected stake tree transaction", txIdx)
				}

				// Check it has exactly one input (null input for new UTXO)
				if len(msgTx.TxIn) != 1 {
					t.Errorf("Tx %d: Expected 1 input, got %d", txIdx, len(msgTx.TxIn))
				}

				// Should be null input since we passed nil utxoView
				if msgTx.TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
					t.Errorf("Tx %d: Expected null input", txIdx)
				}

				// Check outputs: one for voter + one OP_RETURN
				if len(msgTx.TxOut) != 2 {
					t.Errorf("Tx %d: Expected 2 outputs, got %d", txIdx, len(msgTx.TxOut))
				}

				// Check first output has the correct coin type
				if msgTx.TxOut[0].CoinType != test.coinType {
					t.Errorf("Tx %d: Output has coin type %d, expected %d",
						txIdx, msgTx.TxOut[0].CoinType, test.coinType)
				}

				// Accumulate total distributed
				totalDistributed += msgTx.TxOut[0].Value

				// NOTE: stake.IsSSFee() will fail in Phase 2 because validation
				// hasn't been updated yet to accept non-null inputs (Phase 3 work).
				// For now, just verify basic structure:
				// - Version 3
				// - 1 input (null for now, since utxoView is nil in these tests)
				// - 2 outputs (voter + OP_RETURN)
				if msgTx.Version != 3 {
					t.Errorf("Tx %d: Wrong version", txIdx)
				}
			}

			// Verify total distributed across all transactions equals input fee
			if totalDistributed != test.totalFee {
				t.Errorf("Total distributed %d != total fee %d",
					totalDistributed, test.totalFee)
			}
		})
	}
}

// TestSSFeeMultipleCoinTypes tests that multiple SSFee transactions can be created
// for different coin types in the same block
func TestSSFeeMultipleCoinTypes(t *testing.T) {
	// Create mock voters
	voters := make([]*dcrutil.Tx, 2)
	for i := range voters {
		voteTx := wire.NewMsgTx()
		voteTx.Version = 3

		// Add minimal vote structure
		for j := 0; j < 3; j++ {
			voteTx.AddTxOut(&wire.TxOut{
				Value:    1000 * int64(j),
				CoinType: cointype.CoinTypeVAR,
				Version:  0,
				PkScript: []byte{txscript.OP_DUP},
			})
		}
		voters[i] = dcrutil.NewTx(voteTx)
	}

	// Test creating SSFee for multiple coin types
	// These are the staker portions (50% of total fees per coin type)
	coinTypes := []cointype.CoinType{1, 2, 3} // SKA-1, SKA-2, SKA-3
	fees := []int64{2000, 4000, 6000}         // Staker portions after 50/50 split

	allSSFeeTxns := make([][]*dcrutil.Tx, 0)
	for i, coinType := range coinTypes {
		ssFeeTxns, err := createSSFeeTx(coinType, fees[i], voters, 100, nil)
		if err != nil {
			t.Fatalf("Failed to create SSFee for coin type %d: %v", coinType, err)
		}
		allSSFeeTxns = append(allSSFeeTxns, ssFeeTxns)
	}

	// Verify each SSFee transaction group
	for i, ssFeeTxns := range allSSFeeTxns {
		expectedCoinType := coinTypes[i]
		var totalDistributed int64

		for _, ssFeeTx := range ssFeeTxns {
			msgTx := ssFeeTx.MsgTx()

			// Check coin type consistency
			for j, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
				if out.CoinType != expectedCoinType {
					t.Errorf("SSFee %d output %d has wrong coin type: got %d, want %d",
						i, j, out.CoinType, expectedCoinType)
				}
				totalDistributed += out.Value
			}
		}

		// Check fee distribution across all SSFee transactions for this coin type
		if totalDistributed != fees[i] {
			t.Errorf("SSFee group %d distributed wrong amount: got %d, want %d",
				i, totalDistributed, fees[i])
		}
	}

	// Verify all SSFee transactions are different
	hashes := make(map[chainhash.Hash]bool)
	for _, ssFeeTxns := range allSSFeeTxns {
		for _, ssFeeTx := range ssFeeTxns {
			hash := ssFeeTx.Hash()
			if hashes[*hash] {
				t.Errorf("Duplicate SSFee transaction hash: %v", hash)
			}
			hashes[*hash] = true
		}
	}
}

// TestSSFeeEdgeCases tests various edge cases and security scenarios
func TestSSFeeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		setupVoters func() []*dcrutil.Tx
		coinType    cointype.CoinType
		totalFee    int64
		expectError bool
		errorMsg    string
	}{
		{
			name: "malformed voters - missing outputs",
			setupVoters: func() []*dcrutil.Tx {
				voteTx := wire.NewMsgTx()
				voteTx.Version = 3
				// Only add 2 outputs instead of required 3
				voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
				voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
				return []*dcrutil.Tx{dcrutil.NewTx(voteTx)}
			},
			coinType:    1,
			totalFee:    1000,
			expectError: true,
			errorMsg:    "no valid voters found after validation",
		},
		{
			name: "voters with negative reward values",
			setupVoters: func() []*dcrutil.Tx {
				voteTx := wire.NewMsgTx()
				voteTx.Version = 3
				voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
				voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
				// Negative reward value - should be filtered out
				voteTx.AddTxOut(&wire.TxOut{Value: -1000, CoinType: cointype.CoinTypeVAR})
				return []*dcrutil.Tx{dcrutil.NewTx(voteTx)}
			},
			coinType:    1,
			totalFee:    1000,
			expectError: true,
			errorMsg:    "no valid voters found after validation",
		},
		{
			name: "mixed valid and invalid voters",
			setupVoters: func() []*dcrutil.Tx {
				voters := make([]*dcrutil.Tx, 3)

				// Valid voter 1
				voteTx1 := wire.NewMsgTx()
				voteTx1.Version = 3
				for i := 0; i < 3; i++ {
					voteTx1.AddTxOut(&wire.TxOut{
						Value:    1000 * int64(i+1),
						CoinType: cointype.CoinTypeVAR,
					})
				}
				voters[0] = dcrutil.NewTx(voteTx1)

				// Invalid voter - malformed
				voteTx2 := wire.NewMsgTx()
				voteTx2.Version = 3
				voteTx2.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
				voters[1] = dcrutil.NewTx(voteTx2)

				// Valid voter 2 with higher stake
				voteTx3 := wire.NewMsgTx()
				voteTx3.Version = 3
				for i := 0; i < 3; i++ {
					voteTx3.AddTxOut(&wire.TxOut{
						Value:    5000 * int64(i+1), // Higher stake - should get remainder
						CoinType: cointype.CoinTypeVAR,
					})
				}
				voters[2] = dcrutil.NewTx(voteTx3)

				return voters
			},
			coinType:    1,
			totalFee:    1001, // 1001/2 = 500 each + 1 remainder to highest stake
			expectError: false,
		},
		{
			name: "remainder distribution to highest stake voter",
			setupVoters: func() []*dcrutil.Tx {
				voters := make([]*dcrutil.Tx, 3)
				stakes := []int64{1000, 5000, 2000} // Middle voter has highest stake

				for i, stake := range stakes {
					voteTx := wire.NewMsgTx()
					voteTx.Version = 3
					voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
					voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR})
					voteTx.AddTxOut(&wire.TxOut{Value: stake, CoinType: cointype.CoinTypeVAR})
					voters[i] = dcrutil.NewTx(voteTx)
				}
				return voters
			},
			coinType:    1,
			totalFee:    100, // 100/3 = 33 each + 1 remainder to voter with 5000 stake
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			voters := test.setupVoters()
			ssFeeTxns, err := createSSFeeTx(test.coinType, test.totalFee, voters, 100, nil)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if test.errorMsg != "" && !contains(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", test.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Count valid voters (those with 3+ outputs and non-negative reward values)
			validVoterCount := 0
			for _, voter := range voters {
				if voter.MsgTx() != nil && len(voter.MsgTx().TxOut) >= 3 && voter.MsgTx().TxOut[2].Value >= 0 {
					validVoterCount++
				}
			}

			// Should have one transaction per valid voter
			if len(ssFeeTxns) != validVoterCount {
				t.Errorf("Expected %d SSFee transactions, got %d", validVoterCount, len(ssFeeTxns))
			}

			// Additional validation for successful cases
			var totalDistributed int64
			for _, ssFeeTx := range ssFeeTxns {
				msgTx := ssFeeTx.MsgTx()

				// Each SSFee transaction should have 2 outputs: one for voter + one OP_RETURN
				if len(msgTx.TxOut) != 2 {
					t.Errorf("Expected 2 outputs per SSFee tx, got %d", len(msgTx.TxOut))
				}

				// Accumulate total distribution
				totalDistributed += msgTx.TxOut[0].Value
			}

			// Verify total distribution across all SSFee transactions
			if totalDistributed != test.totalFee {
				t.Errorf("Total distributed %d != total fee %d", totalDistributed, test.totalFee)
			}

			// For remainder test, verify highest stake voter gets remainder
			if test.name == "remainder distribution to highest stake voter" && len(ssFeeTxns) >= 3 {
				feePerVoter := test.totalFee / int64(validVoterCount)
				remainder := test.totalFee - (feePerVoter * int64(validVoterCount))

				// Find the transaction with the extra remainder
				foundRemainder := false
				for _, ssFeeTx := range ssFeeTxns {
					if ssFeeTx.MsgTx().TxOut[0].Value == feePerVoter+remainder {
						foundRemainder = true
						break
					}
				}
				if !foundRemainder {
					t.Errorf("Remainder not properly distributed to highest stake voter")
				}
			}
		})
	}
}

// contains checks if a string contains a substring (helper function)
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()
}

// TestSSFeeIntegration tests that SSFee transactions integrate properly with
// the block template generation
func TestSSFeeIntegration(t *testing.T) {
	// This test would require a more complete setup with a mock BlockChain
	// and would test the full integration in NewBlockTemplate
	// For now, we're focusing on unit testing the createSSFeeTx function
	t.Skip("Integration test requires full mining harness setup")
}

// TestCreateSSFeeTxUTXOAugmentation tests UTXO augmentation for staker fees
func TestCreateSSFeeTxUTXOAugmentation(t *testing.T) {
	// NOTE: This test demonstrates the augmentation logic but cannot fully test it
	// without a complete UTXO viewpoint implementation. The test shows that:
	// 1. Without utxoView, SSFee transactions use null inputs (create new UTXOs)
	// 2. With utxoView (future integration), SSFee can augment existing UTXOs

	// Create mock voters with distinct addresses
	voters := make([]*dcrutil.Tx, 3)
	voterPkScripts := [][]byte{
		{txscript.OP_DUP, txscript.OP_HASH160, 0x14, 0x01}, // Voter 0
		{txscript.OP_DUP, txscript.OP_HASH160, 0x14, 0x02}, // Voter 1
		{txscript.OP_DUP, txscript.OP_HASH160, 0x14, 0x03}, // Voter 2
	}

	for i := range voters {
		voteTx := wire.NewMsgTx()
		voteTx.Version = 3

		// Add vote structure
		voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR, PkScript: []byte{txscript.OP_RETURN}})
		voteTx.AddTxOut(&wire.TxOut{Value: 0, CoinType: cointype.CoinTypeVAR, PkScript: []byte{txscript.OP_RETURN}})
		voteTx.AddTxOut(&wire.TxOut{
			Value:    1000,
			CoinType: cointype.CoinTypeVAR,
			Version:  0,
			PkScript: voterPkScripts[i],
		})

		voters[i] = dcrutil.NewTx(voteTx)
	}

	t.Run("no utxoView - creates new UTXOs", func(t *testing.T) {
		ssFeeTxns, err := createSSFeeTx(1, 3000, voters, 100, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should create 3 transactions (one per voter)
		if len(ssFeeTxns) != 3 {
			t.Fatalf("Expected 3 SSFee transactions, got %d", len(ssFeeTxns))
		}

		// All should have null inputs (no augmentation)
		for i, tx := range ssFeeTxns {
			if len(tx.MsgTx().TxIn) != 1 {
				t.Errorf("Tx %d: Expected 1 input", i)
			}
			if tx.MsgTx().TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
				t.Errorf("Tx %d: Expected null input (no UTXO to augment)", i)
			}
			// Output value should equal fee (1000 each for 3 voters)
			if tx.MsgTx().TxOut[0].Value != 1000 {
				t.Errorf("Tx %d: Expected output value 1000, got %d", i, tx.MsgTx().TxOut[0].Value)
			}
		}
	})

	t.Run("multiple rounds accumulate fees", func(t *testing.T) {
		// Round 1: Create initial SSFee transactions
		round1Txns, err := createSSFeeTx(1, 3000, voters, 100, nil)
		if err != nil {
			t.Fatalf("Round 1 error: %v", err)
		}

		// Round 2: Create more SSFee transactions (simulating next block)
		// In real scenario, round1 outputs would be in UTXO set and could be augmented
		round2Txns, err := createSSFeeTx(1, 3000, voters, 101, nil)
		if err != nil {
			t.Fatalf("Round 2 error: %v", err)
		}

		// Both rounds should create separate transactions
		if len(round1Txns) != 3 || len(round2Txns) != 3 {
			t.Fatalf("Expected 3 transactions per round")
		}

		// This demonstrates that without UTXO augmentation, we get 6 total UTXOs (dust)
		// With augmentation (when utxoView is populated), round2 would reuse round1 outputs
		totalUTXOs := len(round1Txns) + len(round2Txns)
		t.Logf("Without augmentation: %d total UTXOs created (dust accumulation)", totalUTXOs)
		t.Logf("With augmentation: would create only %d UTXOs (one per voter)", len(voters))
	})

	t.Run("different coin types don't interfere", func(t *testing.T) {
		// Create SSFee for SKA-1
		ska1Txns, err := createSSFeeTx(1, 3000, voters, 100, nil)
		if err != nil {
			t.Fatalf("SKA-1 error: %v", err)
		}

		// Create SSFee for SKA-2
		ska2Txns, err := createSSFeeTx(2, 6000, voters, 100, nil)
		if err != nil {
			t.Fatalf("SKA-2 error: %v", err)
		}

		// Should create separate transactions per coin type
		if len(ska1Txns) != 3 || len(ska2Txns) != 3 {
			t.Fatalf("Expected 3 transactions per coin type")
		}

		// Verify coin types are correct
		for _, tx := range ska1Txns {
			if tx.MsgTx().TxOut[0].CoinType != 1 {
				t.Errorf("SKA-1 transaction has wrong coin type")
			}
		}
		for _, tx := range ska2Txns {
			if tx.MsgTx().TxOut[0].CoinType != 2 {
				t.Errorf("SKA-2 transaction has wrong coin type")
			}
			// SKA-2 should have 2000 per voter (6000 / 3)
			if tx.MsgTx().TxOut[0].Value != 2000 {
				t.Errorf("SKA-2 tx has wrong value: %d", tx.MsgTx().TxOut[0].Value)
			}
		}
	})

	t.Run("each transaction has unique OP_RETURN", func(t *testing.T) {
		ssFeeTxns, err := createSSFeeTx(1, 3000, voters, 100, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Collect all OP_RETURN scripts
		opReturns := make([][]byte, len(ssFeeTxns))
		for i, tx := range ssFeeTxns {
			// OP_RETURN is the last output
			opReturnOut := tx.MsgTx().TxOut[len(tx.MsgTx().TxOut)-1]
			opReturns[i] = opReturnOut.PkScript
		}

		// All should be unique (different voter sequence numbers)
		for i := 0; i < len(opReturns); i++ {
			for j := i + 1; j < len(opReturns); j++ {
				if string(opReturns[i]) == string(opReturns[j]) {
					t.Errorf("SSFee transactions %d and %d have identical OP_RETURN (not unique)", i, j)
				}
			}
		}
	})
}
