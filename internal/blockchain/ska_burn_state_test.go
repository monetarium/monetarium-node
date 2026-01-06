// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/database"
)

// TestSKABurnStateConnectDisconnect tests basic connect and disconnect of burns.
func TestSKABurnStateConnectDisconnect(t *testing.T) {
	t.Parallel()

	// Create test database
	db, teardown := createTestDB(t, "burnstate_connect_disconnect")
	defer teardown()

	state, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState failed: %v", err)
	}

	// Initial state should be empty
	if amount := state.GetBurnedAmount(1); amount != 0 {
		t.Errorf("Expected 0 burned for SKA-1, got %d", amount)
	}

	// Create burn records
	burns := []SKABurnRecord{
		{
			CoinType: 1,
			Amount:   100000000000, // 1000 coins
			Height:   100,
			TxHash:   [32]byte{1, 2, 3},
			OutIndex: 0,
		},
	}

	// Connect burns in a transaction
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, burns)
	})
	if err != nil {
		t.Fatalf("ConnectSKABurnsTx failed: %v", err)
	}

	// Verify amount was tracked
	if amount := state.GetBurnedAmount(1); amount != 100000000000 {
		t.Errorf("Expected 100000000000 burned for SKA-1, got %d", amount)
	}

	// Disconnect the burns (reorg)
	err = db.Update(func(dbTx database.Tx) error {
		return state.DisconnectSKABurnsTx(dbTx, burns)
	})
	if err != nil {
		t.Fatalf("DisconnectSKABurnsTx failed: %v", err)
	}

	// Verify amount was rolled back
	if amount := state.GetBurnedAmount(1); amount != 0 {
		t.Errorf("Expected 0 burned for SKA-1 after disconnect, got %d", amount)
	}

	// Verify coin type was removed from map
	all := state.GetAllBurnedAmounts()
	if len(all) != 0 {
		t.Errorf("Expected empty map after disconnect to zero, got %d entries", len(all))
	}
}

// TestSKABurnStateMultipleBurnsSameCoinType tests multiple burns for same coin type.
func TestSKABurnStateMultipleBurnsSameCoinType(t *testing.T) {
	t.Parallel()

	db, teardown := createTestDB(t, "burnstate_multiple_same")
	defer teardown()

	state, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState failed: %v", err)
	}

	// Connect first burn
	burns1 := []SKABurnRecord{
		{CoinType: 1, Amount: 50000000000, Height: 100, TxHash: [32]byte{1}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, burns1)
	})
	if err != nil {
		t.Fatalf("ConnectSKABurnsTx #1 failed: %v", err)
	}

	// Verify first burn
	if amount := state.GetBurnedAmount(1); amount != 50000000000 {
		t.Errorf("After burn #1: expected 50000000000, got %d", amount)
	}

	// Connect second burn for same coin type
	burns2 := []SKABurnRecord{
		{CoinType: 1, Amount: 30000000000, Height: 101, TxHash: [32]byte{2}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, burns2)
	})
	if err != nil {
		t.Fatalf("ConnectSKABurnsTx #2 failed: %v", err)
	}

	// Verify cumulative amount
	if amount := state.GetBurnedAmount(1); amount != 80000000000 {
		t.Errorf("After burn #2: expected 80000000000, got %d", amount)
	}

	// Disconnect second burn (partial reorg)
	err = db.Update(func(dbTx database.Tx) error {
		return state.DisconnectSKABurnsTx(dbTx, burns2)
	})
	if err != nil {
		t.Fatalf("DisconnectSKABurnsTx #2 failed: %v", err)
	}

	// Verify amount is back to first burn only
	if amount := state.GetBurnedAmount(1); amount != 50000000000 {
		t.Errorf("After disconnect #2: expected 50000000000, got %d", amount)
	}

	// Disconnect first burn
	err = db.Update(func(dbTx database.Tx) error {
		return state.DisconnectSKABurnsTx(dbTx, burns1)
	})
	if err != nil {
		t.Fatalf("DisconnectSKABurnsTx #1 failed: %v", err)
	}

	// Verify all burns rolled back
	if amount := state.GetBurnedAmount(1); amount != 0 {
		t.Errorf("After disconnect all: expected 0, got %d", amount)
	}
}

// TestSKABurnStateMultipleCoinTypes tests burns for multiple coin types.
func TestSKABurnStateMultipleCoinTypes(t *testing.T) {
	t.Parallel()

	db, teardown := createTestDB(t, "burnstate_multiple_types")
	defer teardown()

	state, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState failed: %v", err)
	}

	// Connect burns for different coin types in same block
	burns := []SKABurnRecord{
		{CoinType: 1, Amount: 100000000000, Height: 100, TxHash: [32]byte{1}, OutIndex: 0},
		{CoinType: 2, Amount: 50000000000, Height: 100, TxHash: [32]byte{2}, OutIndex: 0},
		{CoinType: 255, Amount: 25000000000, Height: 100, TxHash: [32]byte{3}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, burns)
	})
	if err != nil {
		t.Fatalf("ConnectSKABurnsTx failed: %v", err)
	}

	// Verify each coin type
	if amount := state.GetBurnedAmount(1); amount != 100000000000 {
		t.Errorf("SKA-1: expected 100000000000, got %d", amount)
	}
	if amount := state.GetBurnedAmount(2); amount != 50000000000 {
		t.Errorf("SKA-2: expected 50000000000, got %d", amount)
	}
	if amount := state.GetBurnedAmount(255); amount != 25000000000 {
		t.Errorf("SKA-255: expected 25000000000, got %d", amount)
	}

	// Verify GetAllBurnedAmounts
	all := state.GetAllBurnedAmounts()
	if len(all) != 3 {
		t.Errorf("Expected 3 coin types, got %d", len(all))
	}

	// Disconnect the entire block (reorg)
	err = db.Update(func(dbTx database.Tx) error {
		return state.DisconnectSKABurnsTx(dbTx, burns)
	})
	if err != nil {
		t.Fatalf("DisconnectSKABurnsTx failed: %v", err)
	}

	// Verify all coin types rolled back
	if amount := state.GetBurnedAmount(1); amount != 0 {
		t.Errorf("SKA-1 after disconnect: expected 0, got %d", amount)
	}
	if amount := state.GetBurnedAmount(2); amount != 0 {
		t.Errorf("SKA-2 after disconnect: expected 0, got %d", amount)
	}
	if amount := state.GetBurnedAmount(255); amount != 0 {
		t.Errorf("SKA-255 after disconnect: expected 0, got %d", amount)
	}

	all = state.GetAllBurnedAmounts()
	if len(all) != 0 {
		t.Errorf("Expected empty map after disconnect, got %d entries", len(all))
	}
}

// TestSKABurnStatePersistence tests that burn state survives database reload.
func TestSKABurnStatePersistence(t *testing.T) {
	t.Parallel()

	db, teardown := createTestDB(t, "burnstate_persistence")
	defer teardown()

	// Create initial state and add burns
	state1, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState #1 failed: %v", err)
	}

	burns := []SKABurnRecord{
		{CoinType: 1, Amount: 100000000000, Height: 100, TxHash: [32]byte{1}, OutIndex: 0},
		{CoinType: 2, Amount: 50000000000, Height: 100, TxHash: [32]byte{2}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state1.ConnectSKABurnsTx(dbTx, burns)
	})
	if err != nil {
		t.Fatalf("ConnectSKABurnsTx failed: %v", err)
	}

	// Create new state instance (simulates restart)
	state2, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState #2 failed: %v", err)
	}

	// Verify state was loaded from database
	if amount := state2.GetBurnedAmount(1); amount != 100000000000 {
		t.Errorf("SKA-1 after reload: expected 100000000000, got %d", amount)
	}
	if amount := state2.GetBurnedAmount(2); amount != 50000000000 {
		t.Errorf("SKA-2 after reload: expected 50000000000, got %d", amount)
	}
}

// TestSKABurnStateReorgScenario tests a realistic reorg scenario.
func TestSKABurnStateReorgScenario(t *testing.T) {
	t.Parallel()

	db, teardown := createTestDB(t, "burnstate_reorg")
	defer teardown()

	state, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState failed: %v", err)
	}

	// Scenario:
	// 1. Block 100: Burn 1000 SKA-1
	// 2. Block 101: Burn 500 SKA-1
	// 3. Block 102: Burn 200 SKA-2
	// 4. Reorg: Disconnect blocks 102, 101
	// 5. Verify state matches block 100 only

	// Connect block 100
	block100 := []SKABurnRecord{
		{CoinType: 1, Amount: 100000000000, Height: 100, TxHash: [32]byte{1}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, block100)
	})
	if err != nil {
		t.Fatalf("Connect block 100 failed: %v", err)
	}

	// Connect block 101
	block101 := []SKABurnRecord{
		{CoinType: 1, Amount: 50000000000, Height: 101, TxHash: [32]byte{2}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, block101)
	})
	if err != nil {
		t.Fatalf("Connect block 101 failed: %v", err)
	}

	// Connect block 102
	block102 := []SKABurnRecord{
		{CoinType: 2, Amount: 20000000000, Height: 102, TxHash: [32]byte{3}, OutIndex: 0},
	}
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, block102)
	})
	if err != nil {
		t.Fatalf("Connect block 102 failed: %v", err)
	}

	// Verify state before reorg
	if amount := state.GetBurnedAmount(1); amount != 150000000000 {
		t.Errorf("SKA-1 before reorg: expected 150000000000, got %d", amount)
	}
	if amount := state.GetBurnedAmount(2); amount != 20000000000 {
		t.Errorf("SKA-2 before reorg: expected 20000000000, got %d", amount)
	}

	// Reorg: Disconnect blocks 102 and 101 (in reverse order)
	err = db.Update(func(dbTx database.Tx) error {
		if err := state.DisconnectSKABurnsTx(dbTx, block102); err != nil {
			return err
		}
		return state.DisconnectSKABurnsTx(dbTx, block101)
	})
	if err != nil {
		t.Fatalf("Reorg disconnect failed: %v", err)
	}

	// Verify state after reorg (should match block 100 only)
	if amount := state.GetBurnedAmount(1); amount != 100000000000 {
		t.Errorf("SKA-1 after reorg: expected 100000000000, got %d", amount)
	}
	if amount := state.GetBurnedAmount(2); amount != 0 {
		t.Errorf("SKA-2 after reorg: expected 0, got %d", amount)
	}

	// Verify SKA-2 was removed from map
	all := state.GetAllBurnedAmounts()
	if len(all) != 1 {
		t.Errorf("After reorg: expected 1 coin type, got %d", len(all))
	}
	if _, exists := all[cointype.CoinType(2)]; exists {
		t.Error("SKA-2 should not exist in map after reorg")
	}
}

// TestSKABurnStateEmptyOperations tests that empty burn lists are handled correctly.
func TestSKABurnStateEmptyOperations(t *testing.T) {
	t.Parallel()

	db, teardown := createTestDB(t, "burnstate_empty")
	defer teardown()

	state, err := NewSKABurnState(db)
	if err != nil {
		t.Fatalf("NewSKABurnState failed: %v", err)
	}

	// Connect empty list should be no-op
	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, nil)
	})
	if err != nil {
		t.Errorf("ConnectSKABurnsTx with nil failed: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return state.ConnectSKABurnsTx(dbTx, []SKABurnRecord{})
	})
	if err != nil {
		t.Errorf("ConnectSKABurnsTx with empty slice failed: %v", err)
	}

	// Disconnect empty list should be no-op
	err = db.Update(func(dbTx database.Tx) error {
		return state.DisconnectSKABurnsTx(dbTx, nil)
	})
	if err != nil {
		t.Errorf("DisconnectSKABurnsTx with nil failed: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		return state.DisconnectSKABurnsTx(dbTx, []SKABurnRecord{})
	})
	if err != nil {
		t.Errorf("DisconnectSKABurnsTx with empty slice failed: %v", err)
	}

	// State should remain empty
	all := state.GetAllBurnedAmounts()
	if len(all) != 0 {
		t.Errorf("Expected empty state, got %d entries", len(all))
	}
}

// createTestDB creates a test database for burn state testing.
func createTestDB(t *testing.T, name string) (database.DB, func()) {
	t.Helper()

	// Use in-memory database for tests
	db, err := database.Create("ffldb", t.TempDir()+"/"+name, chaincfg.RegNetParams().Net)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	teardown := func() {
		db.Close()
	}

	return db, teardown
}
