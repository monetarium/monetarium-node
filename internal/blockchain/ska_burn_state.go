// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/database"
	"github.com/monetarium/monetarium-node/dcrutil"
)

// SKA burn state management
// This file manages the persistent state for SKA burns including:
// - Total burned amounts per coin type
// - Proper handling of chain reorganizations
// - Database persistence

const (
	// Database bucket for SKA burn state
	// This is stored in the blockchain database for persistence
	skaBurnStateBucketName = "skaburnstate"

	// Current version of the on-disk format
	skaBurnStateFormatVersion = 1

	// Meta key for format version
	skaBurnStateVersionKey = "__meta_version__"
)

// SKABurnState manages the persistent state for SKA burns.
// This tracks the total amount of each SKA coin type that has been
// permanently destroyed through burn transactions.
//
// The state is updated atomically with block connection/disconnection
// to ensure consistency during chain reorganizations.
type SKABurnState struct {
	// Protects concurrent access to state
	mtx sync.RWMutex

	// Total burned amount for each coin type (in atoms)
	// Only SKA coin types (1-255) are tracked, VAR burns are not allowed
	burned map[cointype.CoinType]int64

	// Database handle for persistence
	db database.DB
}

// NewSKABurnState creates a new SKA burn state manager.
func NewSKABurnState(db database.DB) (*SKABurnState, error) {
	state := &SKABurnState{
		burned: make(map[cointype.CoinType]int64),
		db:     db,
	}

	// Load existing state from database
	if err := state.load(); err != nil {
		return nil, fmt.Errorf("failed to load SKA burn state: %w", err)
	}

	return state, nil
}

// GetBurnedAmount returns the total amount burned for the specified coin type.
// Returns 0 if no burns have occurred for this coin type.
func (s *SKABurnState) GetBurnedAmount(coinType cointype.CoinType) int64 {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.burned[coinType]
}

// GetAllBurnedAmounts returns a copy of all burned amounts.
// This is useful for RPC queries and statistics.
func (s *SKABurnState) GetAllBurnedAmounts() map[cointype.CoinType]int64 {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	// Create a copy to avoid external modification
	burnedCopy := make(map[cointype.CoinType]int64)
	for k, v := range s.burned {
		burnedCopy[k] = v
	}

	return burnedCopy
}

// SKABurnRecord represents a burn transaction output in a block.
// This is used during block connection/disconnection to update state.
type SKABurnRecord struct {
	CoinType cointype.CoinType
	Amount   int64
	Height   int64
	TxHash   [32]byte
	OutIndex uint32
}

// ConnectSKABurnsTx updates the SKA burn state when a block is connected,
// using the provided database transaction for atomicity with block updates.
func (s *SKABurnState) ConnectSKABurnsTx(dbTx database.Tx, burns []SKABurnRecord) error {
	if len(burns) == 0 {
		return nil
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Update state for each burn
	for _, burn := range burns {
		s.burned[burn.CoinType] += burn.Amount

		log.Debugf("Connected SKA burn: coin type %d, amount %d at height %d (tx %x:%d)",
			burn.CoinType, burn.Amount, burn.Height, burn.TxHash[:8], burn.OutIndex)
	}

	// Persist to database using the provided transaction
	return s.saveWithTx(dbTx)
}

// DisconnectSKABurnsTx updates the SKA burn state when a block is disconnected,
// using the provided database transaction for atomicity with block updates.
func (s *SKABurnState) DisconnectSKABurnsTx(dbTx database.Tx, burns []SKABurnRecord) error {
	if len(burns) == 0 {
		return nil
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Reverse state for each burn
	for _, burn := range burns {
		s.burned[burn.CoinType] -= burn.Amount

		// Remove entry if balance reaches zero
		if s.burned[burn.CoinType] == 0 {
			delete(s.burned, burn.CoinType)
		}

		log.Debugf("Disconnected SKA burn: coin type %d, amount %d at height %d (tx %x:%d)",
			burn.CoinType, burn.Amount, burn.Height, burn.TxHash[:8], burn.OutIndex)
	}

	// Persist to database using the provided transaction
	return s.saveWithTx(dbTx)
}

// load reads the SKA burn state from the database.
func (s *SKABurnState) load() error {
	err := s.db.View(func(dbTx database.Tx) error {
		bucket := dbTx.Metadata().Bucket([]byte(skaBurnStateBucketName))
		if bucket == nil {
			// No existing state, start fresh
			return nil
		}

		// Check format version first
		var version uint32
		if versionBytes := bucket.Get([]byte(skaBurnStateVersionKey)); versionBytes != nil {
			if len(versionBytes) != 4 {
				return fmt.Errorf("invalid SKA burn state version encoding: expected 4 bytes, got %d", len(versionBytes))
			}
			version = binary.LittleEndian.Uint32(versionBytes)
		} else {
			// Missing version means v1 (for backwards compatibility)
			version = 1
		}

		// Reject unsupported versions
		if version > skaBurnStateFormatVersion {
			return fmt.Errorf("unsupported SKA burn state version %d > %d", version, skaBurnStateFormatVersion)
		}

		// Read all entries from the bucket
		return bucket.ForEach(func(k, v []byte) error {
			// Skip meta keys
			if string(k) == skaBurnStateVersionKey {
				return nil
			}

			if len(k) != 1 {
				return fmt.Errorf("invalid key length in SKA burn state bucket: %d", len(k))
			}

			// Reject invalid coin type 0 (VAR burns are not allowed)
			if k[0] == 0 {
				return fmt.Errorf("invalid coin type 0 found in SKA burn state")
			}

			coinType := cointype.CoinType(k[0])

			// Value format: [amount:8 bytes] (signed int64)
			if len(v) != 8 {
				return fmt.Errorf("invalid value length for coin type %d: %d", coinType, len(v))
			}

			// Parse burned amount
			amount := int64(binary.LittleEndian.Uint64(v))
			s.burned[coinType] = amount

			return nil
		})
	})

	if err != nil {
		return fmt.Errorf("failed to load SKA burn state: %w", err)
	}

	log.Debugf("Loaded SKA burn state: %d coin types tracked", len(s.burned))
	return nil
}

// saveWithTx writes the SKA burn state using the provided transaction.
// This allows the state to be saved atomically with other blockchain updates.
func (s *SKABurnState) saveWithTx(dbTx database.Tx) error {
	meta := dbTx.Metadata()

	// Delete and recreate bucket for clean state (removes any unknown keys)
	if meta.Bucket([]byte(skaBurnStateBucketName)) != nil {
		if err := meta.DeleteBucket([]byte(skaBurnStateBucketName)); err != nil {
			return fmt.Errorf("failed to delete old SKA burn state bucket: %w", err)
		}
	}

	bucket, err := meta.CreateBucket([]byte(skaBurnStateBucketName))
	if err != nil {
		return fmt.Errorf("failed to create SKA burn state bucket: %w", err)
	}

	// Write format version
	versionBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(versionBytes, skaBurnStateFormatVersion)
	if err := bucket.Put([]byte(skaBurnStateVersionKey), versionBytes); err != nil {
		return fmt.Errorf("failed to save format version: %w", err)
	}

	// Save each coin type's burned amount (only non-zero amounts)
	for coinType, amount := range s.burned {
		if amount == 0 {
			continue
		}

		// Create key (1 byte coin type)
		key := []byte{byte(coinType)}

		// Create value (8 bytes amount)
		value := make([]byte, 8)
		binary.LittleEndian.PutUint64(value, uint64(amount))

		// Store in bucket
		if err := bucket.Put(key, value); err != nil {
			return fmt.Errorf("failed to save burn amount for coin type %d: %w", coinType, err)
		}
	}

	return nil
}

// Clear removes all SKA burn state from the database.
// This should only be used during database initialization or recovery.
func (s *SKABurnState) Clear() error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	// Clear in-memory state
	s.burned = make(map[cointype.CoinType]int64)

	// Clear database state
	return s.db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()

		// Delete the entire bucket if it exists
		if meta.Bucket([]byte(skaBurnStateBucketName)) != nil {
			if err := meta.DeleteBucket([]byte(skaBurnStateBucketName)); err != nil {
				return fmt.Errorf("failed to delete SKA burn state bucket: %w", err)
			}
		}

		return nil
	})
}

// extractSKABurnsFromBlock scans a block for SKA burn transactions and extracts
// burn records for state tracking. This is called during block connection/disconnection.
func extractSKABurnsFromBlock(block *dcrutil.Block, blockHeight int64, params *chaincfg.Params) []SKABurnRecord {
	var burns []SKABurnRecord

	for _, tx := range block.Transactions() {
		msgTx := tx.MsgTx()
		txHash := tx.Hash()

		// Check each output for burn scripts
		for outIndex, txOut := range msgTx.TxOut {
			// Only process SKA coin types (1-255)
			if !txOut.CoinType.IsSKA() {
				continue
			}

			// Check if this is a burn script using the standard script detection
			// The params.IsSKABurnScript function validates:
			// - Exact 11-byte length
			// - OP_RETURN opcode (0x6a)
			// - Push length 0x09
			// - "SKA_BURN" marker
			// - Valid SKA coin type (1-255)
			if params.IsSKABurnScript(txOut.PkScript) {
				burns = append(burns, SKABurnRecord{
					CoinType: txOut.CoinType,
					Amount:   txOut.Value,
					Height:   blockHeight,
					TxHash:   *txHash,
					OutIndex: uint32(outIndex),
				})
			}
		}
	}

	return burns
}
