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

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/decred/dcrd/wire"
)

// TestSKAActivation tests the SKA per-coin activation logic.
func TestSKAActivation(t *testing.T) {
	// Use SimNet parameters for testing
	params := chaincfg.SimNetParams()

	tests := []struct {
		name     string
		coinType cointype.CoinType
		expected bool
	}{
		{
			name:     "Active SKA coin type",
			coinType: 1, // SKA-1 is active in simnet params
			expected: true,
		},
		{
			name:     "Inactive SKA coin type",
			coinType: 99, // Not configured in simnet params
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := params.IsSKACoinTypeActive(test.coinType)
			if result != test.expected {
				t.Errorf("IsSKACoinTypeActive(%d): expected %t, got %t",
					test.coinType, test.expected, result)
			}
		})
	}
}

// TestCreateAuthorizedSKAEmissionTransactionValidation tests the validation logic
// for authorized SKA emission transaction creation.
func TestCreateAuthorizedSKAEmissionTransactionValidation(t *testing.T) {
	params := chaincfg.SimNetParams()

	// Get emission key for SKA-1 from chain params
	emissionKey := params.GetSKAEmissionKey(cointype.CoinType(1))
	if emissionKey == nil {
		t.Fatal("No emission key configured for SKA-1 in test params")
	}

	// Helper function to create authorization
	createAuth := func(addresses []string, amounts []int64) *chaincfg.SKAEmissionAuth {
		totalAmount := int64(0)
		for _, amount := range amounts {
			totalAmount += amount
		}
		return &chaincfg.SKAEmissionAuth{
			EmissionKey: emissionKey,
			Signature:   make([]byte, 64), // Dummy signature for validation tests
			Nonce:       1,
			CoinType:    cointype.CoinType(1),
			Amount:      totalAmount,
			Height:      10, // Valid height for SKA-1 in simnet
		}
	}

	tests := []struct {
		name        string
		addresses   []string
		amounts     []int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Mismatched addresses and amounts",
			addresses:   []string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc", "SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			amounts:     []int64{50000000000000}, // Only one amount
			expectError: true,
			errorMsg:    "length mismatch",
		},
		{
			name:        "No addresses",
			addresses:   []string{},
			amounts:     []int64{},
			expectError: true,
			errorMsg:    "no emission addresses",
		},
		{
			name:        "Invalid amount (zero)",
			addresses:   []string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc", "SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			amounts:     []int64{0, 100000000000000},
			expectError: true,
			errorMsg:    "invalid emission amount",
		},
		{
			name:        "Wrong total amount",
			addresses:   []string{"SsWKp7wtdTZYabYFYSc9cnxhwFEjA5g4pFc"},
			amounts:     []int64{50000000000}, // Wrong amount (authorization will have different amount)
			expectError: true,
			errorMsg:    "does not match authorization",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			auth := createAuth(test.addresses, test.amounts)
			// Special case: create authorization mismatch for "Wrong total amount" test
			if test.name == "Wrong total amount" {
				auth.Amount = 99999999999 // Different from the amounts provided
			}
			_, err := CreateAuthorizedSKAEmissionTransaction(auth, test.addresses, test.amounts, params)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got none", test.errorMsg)
					return
				}
				if len(test.errorMsg) > 0 && !containsStr(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestIsSKAEmissionTransaction tests the detection of SKA emission transactions.
func TestIsSKAEmissionTransaction(t *testing.T) {
	tests := []struct {
		name     string
		tx       *wire.MsgTx
		expected bool
	}{
		{
			name: "Valid SKA emission transaction",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{}, // Null hash
						Index: 0xffffffff,       // Null index
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // Contains "SKA"
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: cointype.CoinType(1),
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: true,
		},
		{
			name: "Multiple inputs",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{},
							Index: 0xffffffff,
						},
						SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
					},
					{
						PreviousOutPoint: wire.OutPoint{
							Hash:  chainhash.Hash{0x01}, // Non-null
							Index: 0,
						},
						SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
					},
				},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: cointype.CoinType(1),
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // Multiple inputs not allowed
		},
		{
			name: "Non-null input",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{0x01}, // Non-null
						Index: 0,
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: cointype.CoinType(1),
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // Input not null
		},
		{
			name: "Missing SKA marker",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{0x01, 0x02, 0x03}, // No "SKA"
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: cointype.CoinType(1),
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // Missing SKA marker
		},
		{
			name: "VAR output instead of SKA",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
				}},
				TxOut: []*wire.TxOut{{
					Value:    100000000,
					CoinType: cointype.CoinTypeVAR, // Wrong coin type
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				}},
			},
			expected: false, // VAR output not allowed
		},
		{
			name: "No outputs",
			tx: &wire.MsgTx{
				TxIn: []*wire.TxIn{{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41},
				}},
				TxOut: []*wire.TxOut{}, // No outputs
			},
			expected: false, // Must have outputs
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := wire.IsSKAEmissionTransaction(test.tx)
			if result != test.expected {
				t.Errorf("wire.IsSKAEmissionTransaction: expected %t, got %t", test.expected, result)
			}
		})
	}
}

// TestValidateAuthorizedSKAEmissionTransactionBasic tests the basic validation logic
// of authorized SKA emission transactions (adapted from legacy test).
func TestValidateAuthorizedSKAEmissionTransactionBasic(t *testing.T) {
	params := chaincfg.SimNetParams()
	emissionHeight := params.SKACoins[1].EmissionHeight // Use per-coin emission height

	// Create test private key and set up authorization
	privKey, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}
	pubKey := privKey.PubKey()

	// Initialize emission key in per-coin configuration
	if params.SKACoins[1] == nil {
		params.SKACoins[1] = &chaincfg.SKACoinConfig{CoinType: 1, Active: true}
	}
	params.SKACoins[1].EmissionKey = pubKey

	// Get emission amount from per-coin configuration
	emissionAmount := params.SKACoins[1].EmissionAmounts[0] // Use first emission amount

	// Create authorization for a valid emission
	auth := &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		Nonce:       1,
		CoinType:    1,
		Amount:      emissionAmount,
		Height:      int64(emissionHeight), // Convert to int64
		Timestamp:   time.Now().Unix(),
	}

	// Create a test script for output
	testScript := []byte{0x76, 0xa9, 0x14}                             // OP_DUP OP_HASH160 OP_DATA_20
	testScript = append(testScript, bytes.Repeat([]byte{0x01}, 20)...) // 20-byte hash
	testScript = append(testScript, 0x88, 0xac)                        // OP_EQUALVERIFY OP_CHECKSIG

	amounts := []int64{emissionAmount}

	// Calculate Expiry (emission window end)
	emissionWindow := int64(params.SKACoins[1].EmissionWindow)
	expiry := uint32(int64(emissionHeight) + emissionWindow)

	// Create a valid authorized emission transaction
	validTx := &wire.MsgTx{
		TxIn: []*wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{},
				Index: 0xffffffff,
			},
			SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // "SKA" marker initially
		}},
		LockTime: 0,
		Expiry:   expiry,
	}

	// Add output with test script
	validTx.TxOut = append(validTx.TxOut, &wire.TxOut{
		Value:    amounts[0],
		CoinType: 1, // Use coin type 1 which has authorization
		Version:  0,
		PkScript: testScript,
	})

	// Sign the transaction properly
	txBytes, err := validTx.BytesPrefix()
	if err != nil {
		t.Fatalf("Failed to serialize tx: %v", err)
	}
	txHash := sha256.Sum256(txBytes)

	// Build the signing message
	var msgBuf bytes.Buffer
	msgBuf.WriteString("SKA-EMIT-V2")
	binary.Write(&msgBuf, binary.LittleEndian, uint32(params.Net)) // Use the actual network
	msgBuf.WriteByte(byte(auth.CoinType))
	binary.Write(&msgBuf, binary.LittleEndian, auth.Nonce)
	binary.Write(&msgBuf, binary.LittleEndian, uint64(auth.Height)) // Sign auth.Height for window-based validation
	msgBuf.Write(txHash[:])

	msgHash := sha256.Sum256(msgBuf.Bytes())
	signature := ecdsa.Sign(privKey, msgHash[:])
	auth.Signature = signature.Serialize()

	// Create authorized signature script
	authScript, err := createEmissionAuthScript(auth)
	if err != nil {
		t.Fatalf("Failed to create auth script: %v", err)
	}
	validTx.TxIn[0].SignatureScript = authScript

	tests := []struct {
		name        string
		tx          *wire.MsgTx
		blockHeight int64
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid authorized emission transaction",
			tx:          validTx,
			blockHeight: int64(emissionHeight),
			expectError: false,
		},
		{
			name:        "Wrong block height outside emission window",
			tx:          validTx,
			blockHeight: int64(emissionHeight + 1000), // Way outside emission window
			expectError: true,
			errorMsg:    "invalid height",
		},
	}

	// Create mock chain for testing
	chain := &BlockChain{
		chainParams: params,
		skaEmissionState: &SKAEmissionState{
			nonces:  make(map[cointype.CoinType]uint64),
			emitted: make(map[cointype.CoinType]bool),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAuthorizedSKAEmissionTransaction(test.tx, test.blockHeight, chain, params)

			if test.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got none", test.errorMsg)
					return
				}
				if !containsStr(err.Error(), test.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", test.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// containsStr checks if a string contains a substring.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) &&
		(substr == "" ||
			(len(s) > 0 && (s == substr ||
				(len(s) > len(substr) &&
					(s[:len(substr)] == substr ||
						s[len(s)-len(substr):] == substr ||
						indexStr(s, substr) >= 0)))))
}

// indexStr finds the index of substr in s, returns -1 if not found.
func indexStr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestSKAEmissionWindow tests the emission window functionality
func TestSKAEmissionWindow(t *testing.T) {
	// Create test parameters with emission windows
	params := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				EmissionHeight: 100,
				EmissionWindow: 50, // 50-block window
			},
			2: {
				CoinType:       2,
				EmissionHeight: 200,
				EmissionWindow: 0, // Exact height only
			},
			3: {
				CoinType:       3,
				EmissionHeight: 300,
				EmissionWindow: 100, // 100-block window
			},
		},
	}

	tests := []struct {
		name        string
		blockHeight int64
		coinType    cointype.CoinType
		expected    bool
	}{
		// SKA-1 tests (emission window 100-150)
		{
			name:        "SKA-1 before window",
			blockHeight: 99,
			coinType:    1,
			expected:    false,
		},
		{
			name:        "SKA-1 at window start",
			blockHeight: 100,
			coinType:    1,
			expected:    true,
		},
		{
			name:        "SKA-1 in window middle",
			blockHeight: 125,
			coinType:    1,
			expected:    true,
		},
		{
			name:        "SKA-1 at window end",
			blockHeight: 150,
			coinType:    1,
			expected:    true,
		},
		{
			name:        "SKA-1 after window",
			blockHeight: 151,
			coinType:    1,
			expected:    false,
		},
		// SKA-2 tests (exact height only)
		{
			name:        "SKA-2 before height",
			blockHeight: 199,
			coinType:    2,
			expected:    false,
		},
		{
			name:        "SKA-2 at exact height",
			blockHeight: 200,
			coinType:    2,
			expected:    true,
		},
		{
			name:        "SKA-2 after height",
			blockHeight: 201,
			coinType:    2,
			expected:    false,
		},
		// SKA-3 tests (emission window 300-400)
		{
			name:        "SKA-3 before window",
			blockHeight: 299,
			coinType:    3,
			expected:    false,
		},
		{
			name:        "SKA-3 at window start",
			blockHeight: 300,
			coinType:    3,
			expected:    true,
		},
		{
			name:        "SKA-3 at window end",
			blockHeight: 400,
			coinType:    3,
			expected:    true,
		},
		{
			name:        "SKA-3 after window",
			blockHeight: 401,
			coinType:    3,
			expected:    false,
		},
		// Non-existent coin type
		{
			name:        "Non-existent coin type",
			blockHeight: 100,
			coinType:    99,
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isSKAEmissionWindow(test.blockHeight, test.coinType, params)
			if result != test.expected {
				t.Errorf("isSKAEmissionWindow(%d, %d): expected %t, got %t",
					test.blockHeight, test.coinType, test.expected, result)
			}
		})
	}
}

// TestSKAEmissionWindowActive tests the emission window active detection
func TestSKAEmissionWindowActive(t *testing.T) {
	// Create test parameters with emission windows
	params := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				EmissionHeight: 100,
				EmissionWindow: 50, // 100-150
			},
			2: {
				CoinType:       2,
				EmissionHeight: 200,
				EmissionWindow: 0, // Exact height only
			},
			3: {
				CoinType:       3,
				EmissionHeight: 300,
				EmissionWindow: 100, // 300-400
			},
		},
	}

	tests := []struct {
		name        string
		blockHeight int64
		expected    bool
	}{
		{
			name:        "No windows active",
			blockHeight: 50,
			expected:    false,
		},
		{
			name:        "SKA-1 window active",
			blockHeight: 125,
			expected:    true,
		},
		{
			name:        "SKA-2 exact height active",
			blockHeight: 200,
			expected:    true,
		},
		{
			name:        "SKA-3 window active",
			blockHeight: 350,
			expected:    true,
		},
		{
			name:        "Between windows",
			blockHeight: 250,
			expected:    false,
		},
		{
			name:        "After all windows",
			blockHeight: 500,
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := isSKAEmissionWindowActive(test.blockHeight, params)
			if result != test.expected {
				t.Errorf("isSKAEmissionWindowActive(%d): expected %t, got %t",
					test.blockHeight, test.expected, result)
			}
		})
	}
}

// TestSKAEmissionWindowEdgeCases tests edge cases for emission windows
func TestSKAEmissionWindowEdgeCases(t *testing.T) {
	// Test with empty parameters
	emptyParams := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{},
	}

	// Test with nil parameters
	if isSKAEmissionWindowActive(100, emptyParams) {
		t.Error("Expected false for empty parameters")
	}

	// Test with very large emission window
	largeWindowParams := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				EmissionHeight: 100,
				EmissionWindow: 1000000, // Very large window
			},
		},
	}

	if !isSKAEmissionWindow(500000, 1, largeWindowParams) {
		t.Error("Expected true for large emission window")
	}

	// Test with zero emission height
	zeroHeightParams := &chaincfg.Params{
		SKACoins: map[cointype.CoinType]*chaincfg.SKACoinConfig{
			1: {
				CoinType:       1,
				EmissionHeight: 0,
				EmissionWindow: 100,
			},
		},
	}

	if !isSKAEmissionWindow(50, 1, zeroHeightParams) {
		t.Error("Expected true for zero emission height with window")
	}
}
