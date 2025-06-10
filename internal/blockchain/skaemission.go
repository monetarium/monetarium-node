// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
)

// isSKAEmissionBlock returns whether or not the provided block is the SKA
// emission block as defined by the chain parameters.
func isSKAEmissionBlock(blockHeight int64, chainParams *chaincfg.Params) bool {
	return blockHeight == chainParams.SKAEmissionHeight
}

// isSKAActive returns whether or not SKA transactions are active for the
// provided block height based on the chain parameters.
func isSKAActive(blockHeight int64, chainParams *chaincfg.Params) bool {
	return blockHeight >= chainParams.SKAActivationHeight
}

// CreateSKAEmissionTransaction creates a special SKA emission transaction that
// emits the total SKA supply at the activation height. This is a one-time event.
//
// The emission transaction has the following structure:
// - Single input: null input (similar to coinbase)
// - Multiple outputs: SKA distribution to specified addresses
// - Total output value equals chainParams.SKAEmissionAmount
func CreateSKAEmissionTransaction(emissionAddresses []string, amounts []int64, 
	chainParams *chaincfg.Params) (*wire.MsgTx, error) {
	
	// Validate inputs
	if len(emissionAddresses) != len(amounts) {
		return nil, fmt.Errorf("emission addresses and amounts length mismatch")
	}
	
	if len(emissionAddresses) == 0 {
		return nil, fmt.Errorf("no emission addresses specified")
	}
	
	// Calculate total emission amount
	var totalAmount int64
	for _, amount := range amounts {
		if amount <= 0 {
			return nil, fmt.Errorf("invalid emission amount: %d", amount)
		}
		totalAmount += amount
	}
	
	// Verify total matches chain parameters
	if totalAmount != chainParams.SKAEmissionAmount {
		return nil, fmt.Errorf("total emission amount %d does not match chain parameter %d",
			totalAmount, chainParams.SKAEmissionAmount)
	}
	
	// Create the emission transaction
	tx := &wire.MsgTx{
		SerType:  wire.TxSerializeFull,
		Version:  1,
		LockTime: 0,
		Expiry:   0,
	}
	
	// Add null input (similar to coinbase)
	tx.TxIn = append(tx.TxIn, &wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{}, // All zeros
			Index: 0xffffffff,      // Max value indicates null
			Tree:  wire.TxTreeRegular,
		},
		SignatureScript: []byte{0x01, 0x53, 0x4b, 0x41}, // "SKA" marker
		Sequence:        0xffffffff,
		BlockHeight:     wire.NullBlockHeight,
		BlockIndex:      wire.NullBlockIndex,
		ValueIn:         wire.NullValueIn,
	})
	
	// Add output for each emission address
	for i, addressStr := range emissionAddresses {
		addr, err := stdaddr.DecodeAddress(addressStr, chainParams)
		if err != nil {
			return nil, fmt.Errorf("invalid emission address %s: %v", addressStr, err)
		}
		
		// Create script for the address
		_, pkScript := addr.PaymentScript()
		
		// Add SKA output
		tx.TxOut = append(tx.TxOut, &wire.TxOut{
			Value:    amounts[i],
			CoinType: wire.CoinTypeSKA, // This is an SKA emission
			Version:  0,
			PkScript: pkScript,
		})
	}
	
	return tx, nil
}

// ValidateSKAEmissionTransaction validates that a transaction is a valid SKA
// emission transaction for the given block height and chain parameters.
func ValidateSKAEmissionTransaction(tx *wire.MsgTx, blockHeight int64, 
	chainParams *chaincfg.Params) error {
	
	// Check if this is the correct block for SKA emission
	if !isSKAEmissionBlock(blockHeight, chainParams) {
		return fmt.Errorf("SKA emission transaction at invalid height %d, expected %d",
			blockHeight, chainParams.SKAEmissionHeight)
	}
	
	// Validate transaction structure
	if len(tx.TxIn) != 1 {
		return fmt.Errorf("SKA emission transaction must have exactly 1 input, got %d", 
			len(tx.TxIn))
	}
	
	if len(tx.TxOut) == 0 {
		return fmt.Errorf("SKA emission transaction must have at least 1 output")
	}
	
	// Validate null input (similar to coinbase validation)
	prevOut := tx.TxIn[0].PreviousOutPoint
	if !prevOut.Hash.IsEqual(&chainhash.Hash{}) || prevOut.Index != 0xffffffff {
		return fmt.Errorf("SKA emission transaction input is not null")
	}
	
	// Validate signature script contains SKA marker
	sigScript := tx.TxIn[0].SignatureScript
	if len(sigScript) < 4 || string(sigScript[len(sigScript)-3:]) != "SKA" {
		return fmt.Errorf("SKA emission transaction missing SKA marker in signature script")
	}
	
	// Validate all outputs are SKA outputs
	var totalEmissionAmount int64
	for i, txOut := range tx.TxOut {
		if txOut.CoinType != wire.CoinTypeSKA {
			return fmt.Errorf("SKA emission transaction output %d is not SKA coin type", i)
		}
		
		if txOut.Value <= 0 {
			return fmt.Errorf("SKA emission transaction output %d has invalid amount %d", 
				i, txOut.Value)
		}
		
		if txOut.Value > chainParams.SKAMaxAmount {
			return fmt.Errorf("SKA emission transaction output %d exceeds maximum %d", 
				i, chainParams.SKAMaxAmount)
		}
		
		totalEmissionAmount += txOut.Value
	}
	
	// Validate total emission amount
	if totalEmissionAmount != chainParams.SKAEmissionAmount {
		return fmt.Errorf("SKA emission total %d does not match chain parameter %d",
			totalEmissionAmount, chainParams.SKAEmissionAmount)
	}
	
	// Validate transaction parameters
	if tx.LockTime != 0 {
		return fmt.Errorf("SKA emission transaction must have LockTime 0")
	}
	
	if tx.Expiry != 0 {
		return fmt.Errorf("SKA emission transaction must have Expiry 0")
	}
	
	return nil
}

// IsSKAEmissionTransaction returns whether the given transaction is an SKA
// emission transaction based on its structure.
func IsSKAEmissionTransaction(tx *wire.MsgTx) bool {
	// Must have exactly one input
	if len(tx.TxIn) != 1 {
		return false
	}
	
	// Must have at least one output
	if len(tx.TxOut) == 0 {
		return false
	}
	
	// Input must be null (similar to coinbase)
	prevOut := tx.TxIn[0].PreviousOutPoint
	if !prevOut.Hash.IsEqual(&chainhash.Hash{}) || prevOut.Index != 0xffffffff {
		return false
	}
	
	// Check for SKA marker in signature script
	sigScript := tx.TxIn[0].SignatureScript
	if len(sigScript) < 4 || string(sigScript[len(sigScript)-3:]) != "SKA" {
		return false
	}
	
	// All outputs must be SKA outputs
	for _, txOut := range tx.TxOut {
		if txOut.CoinType != wire.CoinTypeSKA {
			return false
		}
	}
	
	return true
}

// CheckSKAEmissionInBlock validates SKA emission rules for a block at the given height.
// This function enforces:
// 1. SKA emission block must contain exactly one SKA emission transaction
// 2. Non-emission blocks must not contain any SKA emission transactions
// 3. No SKA transactions are allowed before activation height
func CheckSKAEmissionInBlock(block *dcrutil.Block, blockHeight int64, 
	chainParams *chaincfg.Params) error {
	
	isEmissionBlock := isSKAEmissionBlock(blockHeight, chainParams)
	isActive := isSKAActive(blockHeight, chainParams)
	
	var emissionTxCount int
	var skaTxCount int
	
	// Check all transactions in the block
	for i, tx := range block.Transactions() {
		msgTx := tx.MsgTx()
		
		// Count SKA emission transactions
		if IsSKAEmissionTransaction(msgTx) {
			emissionTxCount++
			
			// Validate the emission transaction
			if err := ValidateSKAEmissionTransaction(msgTx, blockHeight, chainParams); err != nil {
				return fmt.Errorf("invalid SKA emission transaction at index %d: %v", i, err)
			}
		}
		
		// Count transactions with SKA outputs (excluding emission transactions)
		if !IsSKAEmissionTransaction(msgTx) {
			for _, txOut := range msgTx.TxOut {
				if txOut.CoinType == wire.CoinTypeSKA {
					skaTxCount++
					break
				}
			}
		}
	}
	
	// Validate emission rules based on block height
	if isEmissionBlock {
		// Emission block must have exactly one emission transaction
		if emissionTxCount != 1 {
			return fmt.Errorf("SKA emission block at height %d must contain exactly 1 emission transaction, got %d",
				blockHeight, emissionTxCount)
		}
	} else {
		// Non-emission blocks must not have emission transactions
		if emissionTxCount > 0 {
			return fmt.Errorf("block at height %d contains %d SKA emission transactions but is not emission block",
				blockHeight, emissionTxCount)
		}
	}
	
	// Before activation, no SKA transactions are allowed (except emission)
	if !isActive && !isEmissionBlock && skaTxCount > 0 {
		return fmt.Errorf("SKA transactions not allowed before activation height %d (current: %d)",
			chainParams.SKAActivationHeight, blockHeight)
	}
	
	return nil
}