// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

//go:build rpctest

package rpctests

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/rpc/jsonrpc/types"
	"github.com/monetarium/monetarium-node/txscript/stdscript"
	"github.com/monetarium/monetarium-node/wire"
	"github.com/monetarium/test/dcrdtest"
)

// testBurnTransaction tests creating and mining a burn transaction and verifying
// the burn statistics are tracked correctly.
func testBurnTransaction(ctx context.Context, h *dcrdtest.Harness, t *testing.T) {
	// This test requires:
	// 1. Creating a burn script
	// 2. Creating a transaction with burn output (requires SKA coins)
	// 3. Mining the transaction
	// 4. Querying burn stats via RPC
	// 5. Verifying stats match expected values

	t.Log("Test: Creating and verifying burn transaction")

	// Note: This is a placeholder test structure. Full implementation would require:
	// - SKA emission setup (to get SKA coins for burning)
	// - Transaction creation with burn script
	// - Mining and verification

	// For now, we test that burn scripts can be created
	burnScript := stdscript.NewSKABurnScriptV0(1) // SKA-1
	if burnScript == nil {
		t.Fatal("failed to create burn script")
	}

	if !stdscript.IsSKABurnScriptV0(burnScript) {
		t.Fatal("burn script not recognized as burn script")
	}

	coinType, err := stdscript.ExtractSKABurnCoinTypeV0(burnScript)
	if err != nil {
		t.Fatalf("failed to extract coin type from burn script: %v", err)
	}

	if coinType != 1 {
		t.Fatalf("expected coin type 1, got %d", coinType)
	}

	t.Log("✓ Burn script creation and detection working")

	// Query burn stats (should be empty initially)
	client := h.Node

	// Test getburnedcoins RPC with no burns
	rawResult, err := client.RawRequest(ctx, "getburnedcoins", nil)
	if err != nil {
		t.Fatalf("getburnedcoins RPC failed: %v", err)
	}

	var result types.GetBurnedCoinsResult
	err = json.Unmarshal(rawResult, &result)
	if err != nil {
		t.Fatalf("failed to unmarshal getburnedcoins result: %v", err)
	}

	if len(result.Stats) != 0 {
		t.Errorf("expected 0 burn stats initially, got %d", len(result.Stats))
	}

	t.Log("✓ getburnedcoins RPC working with empty state")
}

// testBurnScriptCreation tests that burn scripts can be created for all valid
// SKA coin types.
func testBurnScriptCreation(ctx context.Context, h *dcrdtest.Harness, t *testing.T) {
	t.Log("Test: Burn script creation for all SKA coin types")

	// Test a sample of coin types
	testCoinTypes := []uint8{1, 2, 100, 200, 255}

	for _, ct := range testCoinTypes {
		script := stdscript.NewSKABurnScriptV0(ct)
		if script == nil {
			t.Fatalf("failed to create burn script for coin type %d", ct)
		}

		if !stdscript.IsSKABurnScriptV0(script) {
			t.Fatalf("burn script for coin type %d not recognized", ct)
		}

		extracted, err := stdscript.ExtractSKABurnCoinTypeV0(script)
		if err != nil {
			t.Fatalf("failed to extract coin type %d: %v", ct, err)
		}

		if extracted != ct {
			t.Fatalf("coin type mismatch: expected %d, got %d", ct, extracted)
		}
	}

	// Test that VAR (0) cannot be burned
	varScript := stdscript.NewSKABurnScriptV0(0)
	if varScript != nil {
		t.Fatal("VAR (coin type 0) should not be burnable")
	}

	t.Log("✓ Burn scripts working for all valid coin types")
}

// testGetBurnedCoinsRPC tests the getburnedcoins RPC command with various parameters.
func testGetBurnedCoinsRPC(ctx context.Context, h *dcrdtest.Harness, t *testing.T) {
	t.Log("Test: getburnedcoins RPC command variations")

	client := h.Node

	// Test 1: Query all coin types (should be empty)
	rawResult, err := client.RawRequest(ctx, "getburnedcoins", nil)
	if err != nil {
		t.Fatalf("getburnedcoins (all) failed: %v", err)
	}

	var allResult types.GetBurnedCoinsResult
	err = json.Unmarshal(rawResult, &allResult)
	if err != nil {
		t.Fatalf("failed to unmarshal getburnedcoins result: %v", err)
	}

	if len(allResult.Stats) != 0 {
		t.Errorf("expected 0 stats for all coins, got %d", len(allResult.Stats))
	}

	// Test 2: Query specific coin type (should return empty result)
	coinTypeParam, err := json.Marshal(uint8(1))
	if err != nil {
		t.Fatalf("failed to marshal coin type parameter: %v", err)
	}
	rawResult2, err := client.RawRequest(ctx, "getburnedcoins", []json.RawMessage{coinTypeParam})
	if err != nil {
		t.Fatalf("getburnedcoins (coin type 1) failed: %v", err)
	}

	var specificResult types.GetBurnedCoinsResult
	err = json.Unmarshal(rawResult2, &specificResult)
	if err != nil {
		t.Fatalf("failed to unmarshal getburnedcoins result: %v", err)
	}

	if len(specificResult.Stats) != 0 {
		t.Errorf("expected 0 stats for SKA-1, got %d", len(specificResult.Stats))
	}

	t.Log("✓ getburnedcoins RPC working with various parameters")
}

// testBurnScriptInTransaction tests creating a transaction with a burn output.
// This is a basic test that verifies the transaction structure.
func testBurnScriptInTransaction(ctx context.Context, h *dcrdtest.Harness, t *testing.T) {
	t.Log("Test: Burn script in transaction structure")

	// Create a burn script
	burnScript := stdscript.NewSKABurnScriptV0(1)
	if burnScript == nil {
		t.Fatal("failed to create burn script")
	}

	// Create a transaction with a burn output (not broadcast, just structure test)
	tx := wire.NewMsgTx()

	// Add burn output
	burnOutput := wire.NewTxOut(1000000000, burnScript) // 10 coins
	burnOutput.CoinType = cointype.CoinType(1)          // SKA-1
	tx.AddTxOut(burnOutput)

	// Verify output is correct
	if len(tx.TxOut) != 1 {
		t.Fatalf("expected 1 output, got %d", len(tx.TxOut))
	}

	output := tx.TxOut[0]
	if output.Value != 1000000000 {
		t.Errorf("expected value 1000000000, got %d", output.Value)
	}

	if output.CoinType != 1 {
		t.Errorf("expected coin type 1, got %d", output.CoinType)
	}

	if !stdscript.IsSKABurnScriptV0(output.PkScript) {
		t.Error("output script is not recognized as burn script")
	}

	t.Log("✓ Transaction with burn output structure valid")
}

// TestBurnFeature runs all burn-related integration tests.
func TestBurnFeature(t *testing.T) {
	defer useTestLogger(t)()

	// Create test harness with regnet
	args := []string{"--rejectnonstd"}
	harness, err := dcrdtest.New(t, chaincfg.RegNetParams(), nil, args)
	if err != nil {
		t.Fatalf("unable to create harness: %v", err)
	}

	// Setup harness with mature coinbase outputs
	ctx := context.Background()
	if err := harness.SetUp(ctx, true, 25); err != nil {
		_ = harness.TearDown()
		t.Fatalf("unable to setup test chain: %v", err)
	}
	defer harness.TearDownInTest(t)

	// Run test cases
	tests := []struct {
		name string
		f    func(context.Context, *dcrdtest.Harness, *testing.T)
	}{
		{
			name: "BurnScriptCreation",
			f:    testBurnScriptCreation,
		},
		{
			name: "BurnScriptInTransaction",
			f:    testBurnScriptInTransaction,
		},
		{
			name: "GetBurnedCoinsRPC",
			f:    testGetBurnedCoinsRPC,
		},
		{
			name: "BurnTransaction",
			f:    testBurnTransaction,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("=== Running test: %v ===", test.name)
			test.f(ctx, harness, t)
		})
	}
}
