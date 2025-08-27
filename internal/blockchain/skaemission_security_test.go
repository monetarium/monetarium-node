// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
)

// TestSKAEmissionSignatureVerification tests that signature verification
// properly prevents unauthorized emissions and tampering.
func TestSKAEmissionSignatureVerification(t *testing.T) {
	// Setup test chain params
	params := &chaincfg.Params{
		Net:          wire.TestNet3,
		SKAMaxAmount: 10000000000, // 100 million coins max per output
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 100,
			},
		},
	}

	// Generate test keys
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Set up emission key in params
	params.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}

	// Create test addresses and amounts
	addresses := []string{
		"TsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc",
	}
	amounts := []int64{1000000}

	// Create a valid emission transaction
	tx := createTestEmissionTx(t, addresses, amounts, 1, params)

	// Create valid authorization
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       1,
		Amount:      1000000,
		Height:      150, // Within window
	}

	// Sign the transaction properly
	signEmissionTx(t, tx, auth, privKey, params)

	// Create mock blockchain with state
	chain := createMockChain(t, params)

	// Test 1: Valid signature should pass
	err = ValidateAuthorizedSKAEmissionTransaction(tx, 150, chain, params)
	if err != nil {
		t.Errorf("Valid emission failed validation: %v", err)
	}

	// Test 2: Tampered signature should fail
	tamperedAuth := *auth
	tamperedAuth.Signature[0] ^= 0xFF // Flip bits in signature
	tamperedTx := createTestEmissionTx(t, addresses, amounts, 1, params)
	embedAuth(tamperedTx, &tamperedAuth)

	err = ValidateAuthorizedSKAEmissionTransaction(tamperedTx, 150, chain, params)
	if err == nil {
		t.Error("Tampered signature passed validation - CRITICAL SECURITY FAILURE")
	}

	// Test 3: Wrong key should fail
	wrongKey, _ := secp256k1.GeneratePrivateKey()
	wrongAuth := *auth
	wrongAuth.EmissionKey = wrongKey.PubKey()
	wrongTx := createTestEmissionTx(t, addresses, amounts, 1, params)
	signEmissionTx(t, wrongTx, &wrongAuth, wrongKey, params)

	err = ValidateAuthorizedSKAEmissionTransaction(wrongTx, 150, chain, params)
	if err == nil {
		t.Error("Wrong key passed validation - CRITICAL SECURITY FAILURE")
	}
}

// TestSKAEmissionMinerRedirectProtection tests that miners cannot
// redirect emission outputs to different addresses.
func TestSKAEmissionMinerRedirectProtection(t *testing.T) {
	// Setup test chain params
	params := &chaincfg.Params{
		Net:          wire.TestNet3,
		SKAMaxAmount: 10000000000, // 100 million coins max per output
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 100,
			},
		},
	}

	// Generate test keys
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	params.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}

	// Original legitimate addresses
	legitAddresses := []string{
		"TsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc",
	}
	amounts := []int64{1000000}

	// Create and sign legitimate transaction
	legitTx := createTestEmissionTx(t, legitAddresses, amounts, 1, params)
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       1,
		Amount:      1000000,
		Height:      150,
	}
	signEmissionTx(t, legitTx, auth, privKey, params)

	// Extract the signature
	legitSigScript := legitTx.TxIn[0].SignatureScript
	legitAuth, err := extractEmissionAuthorization(legitSigScript)
	if err != nil {
		t.Fatalf("Failed to extract auth: %v", err)
	}

	// ATTACK: Miner tries to redirect to their address
	minerAddresses := []string{
		"TsYcrXcBfUA1TAYiMhYRTkHupEapH7QWNU6", // Different address!
	}

	// Create transaction with miner's address but same signature
	redirectedTx := createTestEmissionTx(t, minerAddresses, amounts, 1, params)
	embedAuth(redirectedTx, legitAuth) // Use legitimate signature

	// Create mock blockchain
	chain := createMockChain(t, params)

	// This MUST fail - signature doesn't match transaction
	err = ValidateAuthorizedSKAEmissionTransaction(redirectedTx, 150, chain, params)
	if err == nil {
		t.Fatal("CRITICAL: Miner redirect attack succeeded! Outputs were changed but validation passed")
	}

	// Verify error mentions signature verification
	// The error should contain either the wrapper message or the actual verification failure
	if !bytes.Contains([]byte(err.Error()), []byte("signature verification failed")) &&
		!bytes.Contains([]byte(err.Error()), []byte("emission signature verification failed")) {
		t.Errorf("Expected signature verification failure, got: %v", err)
	}
}

// TestSKAEmissionNetworkReplayProtection tests that emissions cannot
// be replayed across different networks.
func TestSKAEmissionNetworkReplayProtection(t *testing.T) {
	// Create params for two different networks
	mainnetParams := &chaincfg.Params{
		Net: wire.MainNet,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 100,
			},
		},
	}

	testnetParams := &chaincfg.Params{
		Net: wire.TestNet3,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 100,
			},
		},
	}

	// Use same key on both networks
	privKey, _ := secp256k1.GeneratePrivateKey()
	pubKey := privKey.PubKey()

	mainnetParams.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}
	testnetParams.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}

	// Create emission for mainnet
	addresses := []string{"DsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
	amounts := []int64{1000000}

	mainnetTx := createTestEmissionTx(t, addresses, amounts, 1, mainnetParams)
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       1,
		Amount:      1000000,
		Height:      150,
	}

	// Sign for mainnet
	signEmissionTx(t, mainnetTx, auth, privKey, mainnetParams)

	// Try to replay on testnet
	testnetChain := createMockChain(t, testnetParams)

	// This MUST fail due to network ID mismatch
	err := ValidateAuthorizedSKAEmissionTransaction(mainnetTx, 150, testnetChain, testnetParams)
	if err == nil {
		t.Fatal("CRITICAL: Network replay attack succeeded! Transaction from mainnet accepted on testnet")
	}

	// Should fail signature verification due to different network ID
	if !bytes.Contains([]byte(err.Error()), []byte("signature verification failed")) &&
		!bytes.Contains([]byte(err.Error()), []byte("emission signature verification failed")) {
		t.Errorf("Expected signature verification failure due to network mismatch, got: %v", err)
	}
}

// TestSKAEmissionDuplicateProtection tests that the same coin type
// cannot be emitted twice.
func TestSKAEmissionDuplicateProtection(t *testing.T) {
	params := &chaincfg.Params{
		SKAMaxAmount: 10000000000,
		Net:          wire.TestNet3,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 100,
			},
		},
	}

	// Set up emission key
	privKey, _ := secp256k1.GeneratePrivateKey()
	pubKey := privKey.PubKey()
	params.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}

	// Create mock blockchain with coin type 1 already emitted
	chain := createMockChain(t, params)
	// Mark as already emitted directly without database
	chain.skaEmissionState.nonces[1] = 1
	chain.skaEmissionState.emitted[1] = true

	// Try to emit again with nonce 2
	addresses := []string{"TsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
	amounts := []int64{1000000}

	tx := createTestEmissionTx(t, addresses, amounts, 1, params)
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       2, // Next nonce
		Amount:      1000000,
		Height:      150,
	}
	signEmissionTx(t, tx, auth, privKey, params)

	// Check using the fast lookup function
	alreadyExists := CheckSKAEmissionAlreadyExists(1, chain, params)
	if !alreadyExists {
		t.Error("CheckSKAEmissionAlreadyExists failed to detect existing emission")
	}

	// Should also be caught during block validation
	block := dcrutil.NewBlock(&wire.MsgBlock{
		Transactions: []*wire.MsgTx{tx},
	})

	err := CheckSKAEmissionInBlock(block, 150, chain, params)
	if err == nil {
		t.Fatal("CRITICAL: Duplicate emission accepted!")
	}

	if !bytes.Contains([]byte(err.Error()), []byte("already been emitted")) {
		t.Errorf("Expected 'already been emitted' error, got: %v", err)
	}
}

// TestSKAEmissionNonceValidation tests nonce-based replay protection.
func TestSKAEmissionNonceValidation(t *testing.T) {
	params := &chaincfg.Params{
		SKAMaxAmount: 10000000000,
		Net:          wire.TestNet3,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 100,
			},
		},
	}

	privKey, _ := secp256k1.GeneratePrivateKey()
	pubKey := privKey.PubKey()
	params.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}

	chain := createMockChain(t, params)

	addresses := []string{"TsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
	amounts := []int64{1000000}

	// Test 1: Nonce 0 should fail (must start at 1)
	tx0 := createTestEmissionTx(t, addresses, amounts, 1, params)
	auth0 := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       0, // Invalid!
		Amount:      1000000,
		Height:      150,
	}
	signEmissionTx(t, tx0, auth0, privKey, params)

	err := ValidateAuthorizedSKAEmissionTransaction(tx0, 150, chain, params)
	if err == nil {
		t.Error("Nonce 0 accepted - should require nonce 1")
	}

	// Test 2: Nonce 1 should pass (first emission)
	tx1 := createTestEmissionTx(t, addresses, amounts, 1, params)
	auth1 := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       1, // Correct
		Amount:      1000000,
		Height:      150,
	}
	signEmissionTx(t, tx1, auth1, privKey, params)

	err = ValidateAuthorizedSKAEmissionTransaction(tx1, 150, chain, params)
	if err != nil {
		t.Errorf("Valid nonce 1 rejected: %v", err)
	}

	// Test 3: Nonce 2 should fail (skipping ahead)
	tx2 := createTestEmissionTx(t, addresses, amounts, 1, params)
	auth2 := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		CoinType:    1,
		Nonce:       2, // Skipping!
		Amount:      1000000,
		Height:      150,
	}
	signEmissionTx(t, tx2, auth2, privKey, params)

	err = ValidateAuthorizedSKAEmissionTransaction(tx2, 150, chain, params)
	if err == nil {
		t.Error("Nonce skip accepted - should require sequential nonces")
	}
}

// TestSKAEmissionWindowValidation tests that emissions are only
// valid within their configured windows.
func TestSKAEmissionWindowValidation(t *testing.T) {
	params := &chaincfg.Params{
		SKAMaxAmount: 10000000000,
		Net:          wire.TestNet3,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100,
				EmissionWindow: 50, // Window: 100-150
			},
		},
	}

	privKey, _ := secp256k1.GeneratePrivateKey()
	pubKey := privKey.PubKey()
	params.SKAEmissionKeys = map[cointype.CoinType]*secp256k1.PublicKey{
		1: pubKey,
	}

	chain := createMockChain(t, params)

	addresses := []string{"TsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"}
	amounts := []int64{1000000}

	tests := []struct {
		name        string
		blockHeight int64
		shouldPass  bool
	}{
		{"Before window", 99, false},
		{"Start of window", 100, true},
		{"Middle of window", 125, true},
		{"End of window", 150, true},
		{"After window", 151, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := createTestEmissionTx(t, addresses, amounts, 1, params)
			auth := &chaincfg.SKAEmissionAuth{
				EmissionKey: pubKey,
				CoinType:    1,
				Nonce:       1,
				Amount:      1000000,
				Height:      test.blockHeight,
			}
			signEmissionTx(t, tx, auth, privKey, params)

			err := ValidateAuthorizedSKAEmissionTransaction(tx, test.blockHeight, chain, params)

			if test.shouldPass && err != nil {
				t.Errorf("Valid emission at height %d rejected: %v", test.blockHeight, err)
			}
			if !test.shouldPass && err == nil {
				t.Errorf("Invalid emission at height %d accepted", test.blockHeight)
			}
		})
	}
}

// TestSKAPreActivationProtection tests that SKA transactions are
// properly rejected before the activation height.
func TestSKAPreActivationProtection(t *testing.T) {
	params := &chaincfg.Params{
		SKAMaxAmount:        10000000000,
		Net:                 wire.TestNet3,
		SKAActivationHeight: 200,
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				EmissionHeight: 100, // Before activation!
				EmissionWindow: 50,
			},
		},
	}

	chain := createMockChain(t, params)

	// Create a regular SKA transaction (not emission)
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{1, 2, 3},
				Index: 0,
			},
		}},
		TxOut: []*wire.TxOut{{
			Value:    1000,
			CoinType: 1, // SKA coin type
		}},
	}

	block := dcrutil.NewBlock(&wire.MsgBlock{
		Transactions: []*wire.MsgTx{tx},
	})

	// Test at height 150 (during emission window but before activation)
	err := CheckSKAEmissionInBlock(block, 150, chain, params)
	if err == nil {
		t.Fatal("SKA transaction accepted before activation height")
	}

	if !bytes.Contains([]byte(err.Error()), []byte("not allowed before activation")) {
		t.Errorf("Expected activation error, got: %v", err)
	}

	// Test at height 200 (at activation)
	err = CheckSKAEmissionInBlock(block, 200, chain, params)
	if err != nil {
		t.Errorf("SKA transaction rejected at activation height: %v", err)
	}
}

// Helper functions for tests

func createTestEmissionTx(_ *testing.T, addresses []string, amounts []int64, coinType cointype.CoinType, params *chaincfg.Params) *wire.MsgTx {
	tx := &wire.MsgTx{
		Version:  1,
		LockTime: 0,
		Expiry:   0,
	}

	// Add null input for emission
	tx.TxIn = append(tx.TxIn, &wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0xffffffff,
			Tree:  wire.TxTreeRegular,
		},
		SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // Basic SKA marker
		Sequence:        0xffffffff,
	})

	// Add outputs
	for i, addrStr := range addresses {
		addr, err := stdaddr.DecodeAddress(addrStr, params)
		if err != nil {
			// Use a unique dummy script based on address string for testing
			// This ensures different addresses produce different outputs
			hash := sha256.Sum256([]byte(addrStr))
			tx.TxOut = append(tx.TxOut, &wire.TxOut{
				Value:    amounts[i],
				CoinType: coinType,
				Version:  0,
				PkScript: hash[:20], // Use first 20 bytes of hash as unique script
			})
			continue
		}

		ver, pkScript := addr.PaymentScript()
		tx.TxOut = append(tx.TxOut, &wire.TxOut{
			Value:    amounts[i],
			CoinType: coinType,
			Version:  ver,
			PkScript: pkScript,
		})
	}

	return tx
}

func signEmissionTx(t *testing.T, tx *wire.MsgTx, auth *chaincfg.SKAEmissionAuth, privKey *secp256k1.PrivateKey, params *chaincfg.Params) {
	// Compute the transaction hash
	txBytes, err := tx.BytesPrefix()
	if err != nil {
		t.Fatalf("Failed to serialize tx: %v", err)
	}
	txHash := sha256.Sum256(txBytes)

	// Build the signing message (matching verifyEmissionSignature)
	// SECURITY FIX: Use auth.Height instead of blockHeight for window-based validation
	var msgBuf bytes.Buffer
	msgBuf.WriteString("SKA-EMIT-V2")
	binary.Write(&msgBuf, binary.LittleEndian, uint32(params.Net))
	msgBuf.WriteByte(byte(auth.CoinType))
	binary.Write(&msgBuf, binary.LittleEndian, auth.Nonce)
	binary.Write(&msgBuf, binary.LittleEndian, uint64(auth.Height)) // Sign auth.Height, not current block
	msgBuf.Write(txHash[:])

	msgHash := sha256.Sum256(msgBuf.Bytes())

	// Sign the message
	sig := ecdsa.Sign(privKey, msgHash[:])
	auth.Signature = sig.Serialize()

	// Embed the authorization in the transaction
	embedAuth(tx, auth)
}

func embedAuth(tx *wire.MsgTx, auth *chaincfg.SKAEmissionAuth) {
	var script bytes.Buffer

	// SKA marker
	script.Write([]byte{0x01, 0x53, 0x4b, 0x41})

	// Auth version
	script.WriteByte(0x02)

	// Nonce (8 bytes)
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, auth.Nonce)
	script.Write(nonceBytes)

	// Coin type (1 byte)
	script.WriteByte(uint8(auth.CoinType))

	// Amount (8 bytes)
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, uint64(auth.Amount))
	script.Write(amountBytes)

	// Height (8 bytes)
	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightBytes, uint64(auth.Height))
	script.Write(heightBytes)

	// Public key (33 bytes compressed)
	pubKeyBytes := auth.EmissionKey.SerializeCompressed()
	script.Write(pubKeyBytes)

	// Signature length and signature
	script.WriteByte(uint8(len(auth.Signature)))
	script.Write(auth.Signature)

	tx.TxIn[0].SignatureScript = script.Bytes()
}

func createMockChain(_ *testing.T, params *chaincfg.Params) *BlockChain {
	// Create a minimal mock chain for testing
	state := &SKAEmissionState{
		nonces:  make(map[cointype.CoinType]uint64),
		emitted: make(map[cointype.CoinType]bool),
		db:      nil, // No database for tests
	}

	return &BlockChain{
		chainParams:      params,
		skaEmissionState: state,
	}
}
