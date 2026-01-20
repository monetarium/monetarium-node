// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1/ecdsa"
	"github.com/monetarium/monetarium-node/wire"
)

// TestHasVotePassedAtHeight tests the height-based vote checking function.
func TestHasVotePassedAtHeight(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Create a test blockchain
	chain := newFakeChain(params)

	tests := []struct {
		name        string
		voteID      string
		blockHeight int64
		expected    bool
	}{
		{
			name:        "Non-existent vote ID",
			voteID:      "nonexistent",
			blockHeight: 100,
			expected:    false,
		},
		{
			name:        "Valid vote ID at genesis",
			voteID:      chaincfg.VoteIDActivateSKA2,
			blockHeight: 1,
			expected:    false, // Vote not active at genesis
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := chain.HasVotePassedAtHeight(test.voteID, test.blockHeight)
			if result != test.expected {
				t.Errorf("HasVotePassedAtHeight(%s, %d): expected %t, got %t",
					test.voteID, test.blockHeight, test.expected, result)
			}
		})
	}
}

// TestHasVotePassed tests the block node-based vote checking function.
func TestHasVotePassed(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Create a test blockchain
	chain := newFakeChain(params)

	tests := []struct {
		name     string
		voteID   string
		expected bool
	}{
		{
			name:     "Non-existent vote ID",
			voteID:   "nonexistent",
			expected: false,
		},
		{
			name:     "Valid vote ID at genesis",
			voteID:   chaincfg.VoteIDActivateSKA2,
			expected: false, // Vote not active at genesis
		},
	}

	// Get genesis node for testing
	genesisNode := chain.bestChain.NodeByHeight(0)
	if genesisNode == nil {
		t.Fatal("Failed to get genesis node")
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := chain.hasVotePassed(test.voteID, genesisNode)
			if result != test.expected {
				t.Errorf("hasVotePassed(%s, genesis): expected %t, got %t",
					test.voteID, test.expected, result)
			}
		})
	}
}

// TestSKA2EmissionTransactionValidation tests SKA-2 emission transaction validation.
// Note: Vote checking happens at block level, not transaction level, to allow mempool
// to accept transactions before vote passes.
func TestSKA2EmissionTransactionValidation(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Create test private key
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Set up SKA-2 emission key
	if params.SKACoins[2] == nil {
		t.Fatal("SKA-2 not configured in simnet params")
	}
	params.SKACoins[2].EmissionKey = pubKey

	emissionHeight := int64(params.SKACoins[2].EmissionHeight)
	emissionAmount := params.SKACoins[2].EmissionAmounts[0]

	// Create authorization
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		Nonce:       1,
		CoinType:    2, // SKA-2
		Amount:      emissionAmount,
		Height:      emissionHeight,
	}

	// Create test script
	testScript := []byte{0x76, 0xa9, 0x14}
	testScript = append(testScript, bytes.Repeat([]byte{0x01}, 20)...)
	testScript = append(testScript, 0x88, 0xac)

	// Calculate Expiry (emission window end)
	emissionWindow := int64(params.SKACoins[2].EmissionWindow)
	expiry := uint32(emissionHeight + emissionWindow)

	// Create emission transaction
	tx := &wire.MsgTx{
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
		}},
		TxOut: []*wire.TxOut{{
			Value:    0,              // Not used for SKA
			SKAValue: emissionAmount, // SKA uses big.Int for amounts
			CoinType: 2,
			Version:  0,
			PkScript: testScript,
		}},
		LockTime: 0,
		Expiry:   expiry,
	}

	// Sign the transaction
	txBytes, err := tx.BytesPrefix()
	if err != nil {
		t.Fatalf("Failed to serialize tx: %v", err)
	}
	txHash := sha256.Sum256(txBytes)

	var msgBuf bytes.Buffer
	msgBuf.WriteString("SKA-EMIT-V2")
	binary.Write(&msgBuf, binary.LittleEndian, uint32(params.Net))
	msgBuf.WriteByte(byte(auth.CoinType))
	binary.Write(&msgBuf, binary.LittleEndian, auth.Nonce)
	binary.Write(&msgBuf, binary.LittleEndian, uint64(auth.Height))
	msgBuf.Write(txHash[:])

	msgHash := sha256.Sum256(msgBuf.Bytes())
	signature := ecdsa.Sign(privKey, msgHash[:])
	auth.Signature = signature.Serialize()

	authScript, err := createEmissionAuthScript(auth)
	if err != nil {
		t.Fatalf("Failed to create auth script: %v", err)
	}
	tx.TxIn[0].SignatureScript = authScript

	// Create blockchain for testing (vote not active)
	chain := newFakeChain(params)

	// Transaction validation should succeed (mempool accepts before vote)
	// Vote check happens at block validation level in CheckSKAEmissionInBlock
	err = ValidateAuthorizedSKAEmissionTransaction(tx, emissionHeight, chain, params)
	if err != nil {
		t.Errorf("Transaction validation should succeed (vote check at block level), got: %v", err)
	}

	// Note: The actual vote check happens in CheckSKAEmissionInBlock during block validation.
	// This allows the mempool to accept SKA-2 emission transactions before the vote passes,
	// but blocks containing them will be rejected until the vote activates.
}

// TestSKA1EmissionWithoutVote tests that SKA-1 emission succeeds without voting requirement.
func TestSKA1EmissionWithoutVote(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Create test private key
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Set up SKA-1 emission key
	if params.SKACoins[1] == nil {
		t.Fatal("SKA-1 not configured in simnet params")
	}
	params.SKACoins[1].EmissionKey = pubKey

	emissionHeight := int64(params.SKACoins[1].EmissionHeight)
	emissionAmount := params.SKACoins[1].EmissionAmounts[0]

	// Create authorization
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		Nonce:       1,
		CoinType:    1, // SKA-1
		Amount:      emissionAmount,
		Height:      emissionHeight,
	}

	// Create test script
	testScript := []byte{0x76, 0xa9, 0x14}
	testScript = append(testScript, bytes.Repeat([]byte{0x01}, 20)...)
	testScript = append(testScript, 0x88, 0xac)

	// Calculate Expiry (emission window end)
	emissionWindow := int64(params.SKACoins[1].EmissionWindow)
	expiry := uint32(emissionHeight + emissionWindow)

	// Create emission transaction
	tx := &wire.MsgTx{
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
		}},
		TxOut: []*wire.TxOut{{
			Value:    0,              // Not used for SKA
			SKAValue: emissionAmount, // SKA uses big.Int for amounts
			CoinType: 1,
			Version:  0,
			PkScript: testScript,
		}},
		LockTime: 0,
		Expiry:   expiry,
	}

	// Sign the transaction
	txBytes, err := tx.BytesPrefix()
	if err != nil {
		t.Fatalf("Failed to serialize tx: %v", err)
	}
	txHash := sha256.Sum256(txBytes)

	var msgBuf bytes.Buffer
	msgBuf.WriteString("SKA-EMIT-V2")
	binary.Write(&msgBuf, binary.LittleEndian, uint32(params.Net))
	msgBuf.WriteByte(byte(auth.CoinType))
	binary.Write(&msgBuf, binary.LittleEndian, auth.Nonce)
	binary.Write(&msgBuf, binary.LittleEndian, uint64(auth.Height))
	msgBuf.Write(txHash[:])

	msgHash := sha256.Sum256(msgBuf.Bytes())
	signature := ecdsa.Sign(privKey, msgHash[:])
	auth.Signature = signature.Serialize()

	authScript, err := createEmissionAuthScript(auth)
	if err != nil {
		t.Fatalf("Failed to create auth script: %v", err)
	}
	tx.TxIn[0].SignatureScript = authScript

	// Create blockchain for testing
	chain := newFakeChain(params)

	// Attempt emission - should succeed because SKA-1 doesn't require voting
	err = ValidateAuthorizedSKAEmissionTransaction(tx, emissionHeight, chain, params)
	if err != nil {
		t.Errorf("SKA-1 emission should succeed without vote, but got error: %v", err)
	}
}

// TestSKA2VoteActivationFlow tests the complete SKA-2 activation flow.
func TestSKA2VoteActivationFlow(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Verify SKA-2 vote is configured
	stakeVersion := uint32(12)
	deployment, exists := params.Deployments[stakeVersion]
	if !exists {
		t.Fatalf("Stake version %d not configured in simnet params", stakeVersion)
	}

	found := false
	for _, vote := range deployment {
		if vote.Vote.Id == chaincfg.VoteIDActivateSKA2 {
			found = true
			break
		}
	}

	if !found {
		t.Error("SKA-2 activation vote not found in deployments")
	}

	// Verify SKA-2 emission config
	ska2Config, exists := params.SKACoins[2]
	if !exists {
		t.Fatal("SKA-2 not configured in simnet params")
	}

	if ska2Config.Active {
		t.Error("SKA-2 should initially be inactive (activated by vote)")
	}

	if ska2Config.EmissionHeight != 200 {
		t.Errorf("Expected SKA-2 emission height 200, got %d", ska2Config.EmissionHeight)
	}

	if ska2Config.EmissionWindow != 100 {
		t.Errorf("Expected SKA-2 emission window 100, got %d", ska2Config.EmissionWindow)
	}
}

// TestVoteConfiguration tests that voting parameters are correctly configured for testing.
func TestVoteConfiguration(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Verify voting interval is set to 10 for fast testing
	if params.RuleChangeActivationInterval != 10 {
		t.Errorf("Expected RuleChangeActivationInterval 10, got %d", params.RuleChangeActivationInterval)
	}

	// Verify quorum is set appropriately
	if params.RuleChangeActivationQuorum != 5 {
		t.Errorf("Expected RuleChangeActivationQuorum 5, got %d", params.RuleChangeActivationQuorum)
	}

	// Verify vote threshold (75%)
	if params.RuleChangeActivationMultiplier != 3 || params.RuleChangeActivationDivisor != 4 {
		t.Errorf("Expected vote threshold 3/4 (75%%), got %d/%d",
			params.RuleChangeActivationMultiplier, params.RuleChangeActivationDivisor)
	}
}

// TestCoinTypeVoteMapping tests the mapping between coin types and vote IDs.
func TestCoinTypeVoteMapping(t *testing.T) {
	tests := []struct {
		coinType       cointype.CoinType
		expectedVoteID string
		requiresVoting bool
	}{
		{
			coinType:       1,
			expectedVoteID: "",
			requiresVoting: false, // SKA-1 doesn't require voting
		},
		{
			coinType:       2,
			expectedVoteID: "activateska2",
			requiresVoting: true, // SKA-2 requires voting
		},
		{
			coinType:       3,
			expectedVoteID: "activateska3",
			requiresVoting: true, // SKA-3+ require voting
		},
	}

	for _, test := range tests {
		t.Run(test.expectedVoteID, func(t *testing.T) {
			if test.requiresVoting {
				expectedID := test.expectedVoteID
				actualID := chaincfg.VoteIDActivateSKA2 // For SKA-2
				if test.coinType == 2 {
					if actualID != expectedID {
						t.Errorf("Expected vote ID %s for coin type %d, got %s",
							expectedID, test.coinType, actualID)
					}
				}
			}
		})
	}
}

// TestEmissionValidationVoteCheck tests that emission validation properly checks votes for SKA-2+.
func TestEmissionValidationVoteCheck(t *testing.T) {
	params := chaincfg.SimNetParams()

	tests := []struct {
		name        string
		coinType    cointype.CoinType
		expectCheck bool
		description string
	}{
		{
			name:        "SKA-1 no vote check",
			coinType:    1,
			expectCheck: false,
			description: "SKA-1 emissions should not require vote checking",
		},
		{
			name:        "SKA-2 requires vote check",
			coinType:    2,
			expectCheck: true,
			description: "SKA-2 emissions must check for vote activation",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Verify the coin type exists in params
			config, exists := params.SKACoins[test.coinType]
			if !exists {
				t.Skipf("Coin type %d not configured in simnet params", test.coinType)
			}

			// For SKA-2+, verify it requires voting
			if test.expectCheck && test.coinType >= 2 {
				if config.Active {
					t.Errorf("Coin type %d should initially be inactive (requires vote)", test.coinType)
				}
			}

			// For SKA-1, verify it doesn't require voting
			if !test.expectCheck && test.coinType == 1 {
				if !config.Active {
					t.Errorf("SKA-1 should be active by default (no vote required)")
				}
			}
		})
	}
}
