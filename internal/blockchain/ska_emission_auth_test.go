// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1/ecdsa"
	"github.com/monetarium/monetarium-node/wire"
)

// TestEmissionAuthorizationBasic tests the basic emission authorization functionality
func TestEmissionAuthorizationBasic(t *testing.T) {
	// Create test parameters
	params := &chaincfg.Params{
		Net: wire.MainNet, // Set network for signing verification
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				Active:         true,
				EmissionHeight: 100,
				EmissionWindow: 4320,
				EmissionAddresses: []string{
					"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc", // Treasury
				},
				EmissionAmounts: []int64{
					1000000, // 1M atoms to treasury
				},
			},
		},
	}

	// Generate test keys
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Initialize emission key in per-coin configuration
	params.SKACoins[1].EmissionKey = pubKey

	// Create test authorization
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		Signature:   nil, // Will be set after signing
		Nonce:       1,
		CoinType:    1,
		Amount:      1000000,
		Height:      100,
		Timestamp:   time.Now().Unix(),
	}

	// Create a test address for params
	// Generate a simple test script for the address
	testScript := []byte{0x76, 0xa9, 0x14}                             // OP_DUP OP_HASH160 OP_DATA_20
	testScript = append(testScript, bytes.Repeat([]byte{0x01}, 20)...) // 20-byte hash
	testScript = append(testScript, 0x88, 0xac)                        // OP_EQUALVERIFY OP_CHECKSIG

	amounts := []int64{1000000}

	// Create emission transaction
	tx := &wire.MsgTx{
		SerType:  wire.TxSerializeFull,
		Version:  1,
		LockTime: 0,
		Expiry:   0,
	}

	// Add null input
	tx.TxIn = append(tx.TxIn, &wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
			Tree:  wire.TxTreeRegular,
		},
		SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // "SKA" marker
		Sequence:        0xffffffff,
	})

	// Add output with test script
	tx.TxOut = append(tx.TxOut, &wire.TxOut{
		Value:    amounts[0],
		CoinType: auth.CoinType,
		Version:  0,
		PkScript: testScript,
	})

	// Sign the transaction properly
	txBytes, err := tx.BytesPrefix()
	if err != nil {
		t.Fatalf("Failed to serialize tx: %v", err)
	}
	txHash := sha256.Sum256(txBytes)

	// Build the signing message
	var msgBuf bytes.Buffer
	msgBuf.WriteString("SKA-EMIT-V2")
	binary.Write(&msgBuf, binary.LittleEndian, uint32(wire.MainNet)) // Default params uses MainNet
	msgBuf.WriteByte(byte(auth.CoinType))
	binary.Write(&msgBuf, binary.LittleEndian, auth.Nonce)
	binary.Write(&msgBuf, binary.LittleEndian, uint64(auth.Height)) // Sign auth.Height for window-based validation
	msgBuf.Write(txHash[:])

	msgHash := sha256.Sum256(msgBuf.Bytes())
	signature := ecdsa.Sign(privKey, msgHash[:])
	auth.Signature = signature.Serialize()

	// Embed authorization in transaction
	var script bytes.Buffer
	script.Write([]byte{0x01, 0x53, 0x4b, 0x41}) // SKA marker
	script.WriteByte(0x02)                       // Version
	binary.Write(&script, binary.LittleEndian, auth.Nonce)
	script.WriteByte(byte(auth.CoinType))
	script.WriteByte(33) // Public key length
	script.Write(auth.EmissionKey.SerializeCompressed())
	script.WriteByte(byte(len(auth.Signature)))
	script.Write(auth.Signature)
	tx.TxIn[0].SignatureScript = script.Bytes()

	// Create mock chain for testing
	chain := &BlockChain{
		chainParams: params,
		skaEmissionState: &SKAEmissionState{
			nonces:  make(map[cointype.CoinType]uint64),
			emitted: make(map[cointype.CoinType]bool),
		},
	}

	// Test emission authorization validation
	if err := validateEmissionAuthorization(auth, chain, params); err != nil {
		t.Errorf("Valid authorization failed validation: %v", err)
	}

	// Test invalid nonce (replay protection)
	authInvalidNonce := *auth
	authInvalidNonce.Nonce = 0 // Should be 1
	if err := validateEmissionAuthorization(&authInvalidNonce, chain, params); err == nil {
		t.Error("Invalid nonce should have failed validation")
	}

	// Test unauthorized key
	wrongPrivKey, _ := secp256k1.GeneratePrivateKey()
	wrongPubKey := wrongPrivKey.PubKey()
	authWrongKey := *auth
	authWrongKey.EmissionKey = wrongPubKey
	if err := validateEmissionAuthorization(&authWrongKey, chain, params); err == nil {
		t.Error("Wrong key should have failed validation")
	}

	// Test unauthorized coin type
	authWrongCoinType := *auth
	authWrongCoinType.CoinType = 2 // Not configured in params
	if err := validateEmissionAuthorization(&authWrongCoinType, chain, params); err == nil {
		t.Error("Wrong coin type should have failed validation")
	}
}

// TestEmissionAuthorizationScript tests script creation and parsing
func TestEmissionAuthorizationScript(t *testing.T) {
	// Generate test key
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Create test authorization
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		Signature:   []byte{1, 2, 3, 4, 5}, // Dummy signature for testing
		Nonce:       12345,
		CoinType:    1,
		Amount:      1000000,
		Height:      100,
		Timestamp:   time.Now().Unix(),
	}

	// Create authorization script
	script, err := createEmissionAuthScript(auth)
	if err != nil {
		t.Fatalf("Failed to create auth script: %v", err)
	}

	// Parse the script back
	parsedAuth, err := extractEmissionAuthorization(script)
	if err != nil {
		t.Fatalf("Failed to extract auth from script: %v", err)
	}

	// Verify extracted data matches original
	if parsedAuth.Nonce != auth.Nonce {
		t.Errorf("Nonce mismatch: got %d, want %d", parsedAuth.Nonce, auth.Nonce)
	}

	if parsedAuth.CoinType != auth.CoinType {
		t.Errorf("CoinType mismatch: got %d, want %d", parsedAuth.CoinType, auth.CoinType)
	}

	if !parsedAuth.EmissionKey.IsEqual(auth.EmissionKey) {
		t.Error("EmissionKey mismatch")
	}

	if len(parsedAuth.Signature) != len(auth.Signature) {
		t.Errorf("Signature length mismatch: got %d, want %d",
			len(parsedAuth.Signature), len(auth.Signature))
	}
}

// TestUnauthorizedEmissionPrevention tests various unauthorized emission scenarios
func TestUnauthorizedEmissionPrevention(t *testing.T) {
	// Create test parameters with no emission keys configured
	params := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				Active:         true,
				EmissionHeight: 100,
				EmissionWindow: 4320,
				EmissionAddresses: []string{
					"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc", // Treasury
				},
				EmissionAmounts: []int64{
					1000000, // 1M atoms to treasury
				},
				// EmissionKey: nil, // No key configured for this test
			},
		},
	}

	// Test 1: No emission key configured
	privKey, _ := secp256k1.GeneratePrivateKey()
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: privKey.PubKey(),
		CoinType:    1,
		Nonce:       1,
	}

	// Create mock chain for testing
	chain := &BlockChain{
		chainParams: params,
		skaEmissionState: &SKAEmissionState{
			nonces:  make(map[cointype.CoinType]uint64),
			emitted: make(map[cointype.CoinType]bool),
		},
	}

	if err := validateEmissionAuthorization(auth, chain, params); err == nil {
		t.Error("Should fail when no emission key is configured")
	}

	// Test 2: Configure a key and test replay protection
	params.SKACoins[1].EmissionKey = privKey.PubKey()
	// Set nonce in blockchain state instead of params
	chain.skaEmissionState.nonces[1] = 5 // Simulate 5 emissions already done

	// Should fail with nonce 5 (replay)
	auth.Nonce = 5
	if err := validateEmissionAuthorization(auth, chain, params); err == nil {
		t.Error("Should fail with replayed nonce")
	}

	// Should fail with nonce 7 (skipped nonce)
	auth.Nonce = 7
	if err := validateEmissionAuthorization(auth, chain, params); err == nil {
		t.Error("Should fail with skipped nonce")
	}

	// Should succeed with nonce 6 (next valid nonce)
	auth.Nonce = 6
	if err := validateEmissionAuthorization(auth, chain, params); err != nil {
		t.Errorf("Should succeed with next valid nonce: %v", err)
	}
}
