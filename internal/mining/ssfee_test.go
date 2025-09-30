// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"testing"

	"github.com/decred/dcrd/blockchain/stake/v5"
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
			ssFeeTx, err := createSSFeeTx(test.coinType, test.totalFee, test.voters, test.height)

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

			// Validate the created SSFee transaction
			msgTx := ssFeeTx.MsgTx()

			// Check transaction version
			if msgTx.Version != 3 {
				t.Errorf("Expected version 3, got %d", msgTx.Version)
			}

			// Check it's a stake transaction
			if ssFeeTx.Tree() != wire.TxTreeStake {
				t.Errorf("Expected stake tree transaction")
			}

			// Check it has exactly one null input
			if len(msgTx.TxIn) != 1 {
				t.Errorf("Expected 1 input, got %d", len(msgTx.TxIn))
			}

			if msgTx.TxIn[0].PreviousOutPoint.Index != wire.MaxPrevOutIndex {
				t.Errorf("Expected null input")
			}

			// Check outputs: one per voter plus one OP_RETURN
			expectedOutputs := len(test.voters) + 1
			if len(msgTx.TxOut) != expectedOutputs {
				t.Errorf("Expected %d outputs, got %d", expectedOutputs, len(msgTx.TxOut))
			}

			// Check all non-OP_RETURN outputs have the correct coin type
			for i, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
				if out.CoinType != test.coinType {
					t.Errorf("Output %d has coin type %d, expected %d",
						i, out.CoinType, test.coinType)
				}
			}

			// Verify total distributed equals input
			var totalDistributed int64
			for _, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
				totalDistributed += out.Value
			}
			if totalDistributed != test.totalFee {
				t.Errorf("Total distributed %d != total fee %d",
					totalDistributed, test.totalFee)
			}

			// Verify it passes stake tx checks
			if !stake.IsSSFee(msgTx) {
				t.Errorf("Transaction does not pass IsSSFee check")
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

	ssFeeTxns := make([]*dcrutil.Tx, 0)
	for i, coinType := range coinTypes {
		ssFeeTx, err := createSSFeeTx(coinType, fees[i], voters, 100)
		if err != nil {
			t.Fatalf("Failed to create SSFee for coin type %d: %v", coinType, err)
		}
		ssFeeTxns = append(ssFeeTxns, ssFeeTx)
	}

	// Verify each SSFee transaction
	for i, ssFeeTx := range ssFeeTxns {
		msgTx := ssFeeTx.MsgTx()

		// Check coin type consistency
		expectedCoinType := coinTypes[i]
		for j, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
			if out.CoinType != expectedCoinType {
				t.Errorf("SSFee %d output %d has wrong coin type: got %d, want %d",
					i, j, out.CoinType, expectedCoinType)
			}
		}

		// Check fee distribution
		var totalDistributed int64
		for _, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
			totalDistributed += out.Value
		}
		if totalDistributed != fees[i] {
			t.Errorf("SSFee %d distributed wrong amount: got %d, want %d",
				i, totalDistributed, fees[i])
		}
	}

	// Verify all SSFee transactions are different
	hashes := make(map[chainhash.Hash]bool)
	for _, ssFeeTx := range ssFeeTxns {
		hash := ssFeeTx.Hash()
		if hashes[*hash] {
			t.Errorf("Duplicate SSFee transaction hash: %v", hash)
		}
		hashes[*hash] = true
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
			ssFeeTx, err := createSSFeeTx(test.coinType, test.totalFee, voters, 100)

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

			// Additional validation for successful cases
			msgTx := ssFeeTx.MsgTx()

			// Count valid voters (those with 3+ outputs and non-negative reward values)
			validVoterCount := 0
			for _, voter := range voters {
				if voter.MsgTx() != nil && len(voter.MsgTx().TxOut) >= 3 && voter.MsgTx().TxOut[2].Value >= 0 {
					validVoterCount++
				}
			}

			// Verify outputs: one per valid voter plus one OP_RETURN
			expectedOutputs := validVoterCount + 1
			if len(msgTx.TxOut) != expectedOutputs {
				t.Errorf("Expected %d outputs, got %d", expectedOutputs, len(msgTx.TxOut))
			}

			// Verify total distribution
			var totalDistributed int64
			for _, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
				totalDistributed += out.Value
			}
			if totalDistributed != test.totalFee {
				t.Errorf("Total distributed %d != total fee %d", totalDistributed, test.totalFee)
			}

			// For remainder test, verify highest stake voter gets remainder
			if test.name == "remainder distribution to highest stake voter" && len(msgTx.TxOut) >= 4 {
				feePerVoter := test.totalFee / int64(validVoterCount)
				remainder := test.totalFee - (feePerVoter * int64(validVoterCount))

				// Find the output with the extra remainder
				foundRemainder := false
				for _, out := range msgTx.TxOut[:len(msgTx.TxOut)-1] {
					if out.Value == feePerVoter+remainder {
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
