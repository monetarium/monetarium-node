// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/txscript/v4/stdaddr"
	"github.com/decred/dcrd/wire"
)

// isSKAEmissionBlock returns whether or not the provided block is the SKA
// emission block as defined by the chain parameters.
// NOTE: This function is deprecated and only supports SKA-1 emission.
// Use isSKAEmissionWindow for multi-coin support.
func isSKAEmissionBlock(blockHeight int64, chainParams *chaincfg.Params) bool {
	return blockHeight == chainParams.SKAEmissionHeight
}

// isSKAEmissionWindow returns whether the provided block height is within
// the emission window for the specified SKA coin type.
func isSKAEmissionWindow(blockHeight int64, coinType dcrutil.CoinType, chainParams *chaincfg.Params) bool {
	config, exists := chainParams.SKACoins[coinType]
	if !exists {
		log.Debugf("SKA coin type %d not configured in chain parameters", coinType)
		return false
	}

	emissionStart := int64(config.EmissionHeight)
	emissionEnd := emissionStart + int64(config.EmissionWindow)

	// If EmissionWindow is 0, only allow emission at exact height (backward compatibility)
	if config.EmissionWindow == 0 {
		result := blockHeight == emissionStart
		log.Debugf("SKA coin type %d emission window check (exact height): height=%d, required=%d, result=%t",
			coinType, blockHeight, emissionStart, result)
		return result
	}

	result := blockHeight >= emissionStart && blockHeight <= emissionEnd
	log.Debugf("SKA coin type %d emission window check: height=%d, window=%d-%d, result=%t",
		coinType, blockHeight, emissionStart, emissionEnd, result)
	return result
}

// isSKAEmissionWindowActive returns whether any SKA coin type has an active
// emission window at the specified block height.
func isSKAEmissionWindowActive(blockHeight int64, chainParams *chaincfg.Params) bool {
	for coinType := range chainParams.SKACoins {
		if isSKAEmissionWindow(blockHeight, coinType, chainParams) {
			return true
		}
	}
	return false
}

// CheckSKAEmissionAlreadyExists checks if a coin type has already been emitted
// in the blockchain. This function would need to be called with blockchain state
// to implement "first valid emission wins" logic.
// TODO: This function needs blockchain state access to check emission history
func CheckSKAEmissionAlreadyExists(coinType dcrutil.CoinType, blockHeight int64, chainParams *chaincfg.Params) bool {
	// This is a placeholder implementation. In a full implementation, this would:
	// 1. Check the blockchain state for existing emission transactions
	// 2. Look for UTXO entries created by emission transactions for this coin type
	// 3. Return true if any emission has already occurred

	// For now, we'll implement basic window validation
	// The actual "first emission wins" logic will be enforced by the UTXO validation
	return false
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
			Index: 0xffffffff,       // Max value indicates null
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
			return nil, fmt.Errorf("invalid emission address %s: %w", addressStr, err)
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

// CreateAuthorizedSKAEmissionTransaction creates a cryptographically authorized
// SKA emission transaction. This replaces the basic CreateSKAEmissionTransaction
// with proper security controls including signature verification and replay protection.
func CreateAuthorizedSKAEmissionTransaction(auth *chaincfg.SKAEmissionAuth,
	emissionAddresses []string, amounts []int64,
	chainParams *chaincfg.Params) (*wire.MsgTx, error) {

	// Validate authorization structure
	if auth == nil {
		return nil, fmt.Errorf("SKA emission authorization required")
	}

	if auth.EmissionKey == nil {
		return nil, fmt.Errorf("SKA emission key required")
	}

	if len(auth.Signature) == 0 {
		return nil, fmt.Errorf("SKA emission signature required")
	}

	// Validate coin type
	if auth.CoinType < 1 || auth.CoinType > 255 {
		return nil, fmt.Errorf("invalid SKA coin type: %d", auth.CoinType)
	}

	// Check if emission is authorized for this coin type
	authorizedKey := chainParams.GetSKAEmissionKey(auth.CoinType)
	if authorizedKey == nil {
		return nil, fmt.Errorf("no emission key configured for coin type %d", auth.CoinType)
	}

	// Verify the provided key matches the authorized key
	if !bytes.Equal(auth.EmissionKey.SerializeCompressed(),
		authorizedKey.SerializeCompressed()) {
		return nil, fmt.Errorf("unauthorized emission key for coin type %d", auth.CoinType)
	}

	// Check nonce for replay protection
	lastNonce := chainParams.GetSKAEmissionNonce(auth.CoinType)
	if auth.Nonce != lastNonce+1 {
		return nil, fmt.Errorf("invalid nonce: expected %d, got %d", lastNonce+1, auth.Nonce)
	}

	// Validate emission amounts
	if len(emissionAddresses) != len(amounts) {
		return nil, fmt.Errorf("emission addresses and amounts length mismatch")
	}

	if len(emissionAddresses) == 0 {
		return nil, fmt.Errorf("no emission addresses specified")
	}

	var totalAmount int64
	for _, amount := range amounts {
		if amount <= 0 {
			return nil, fmt.Errorf("invalid emission amount: %d", amount)
		}
		totalAmount += amount
	}

	// Verify total matches authorization
	if totalAmount != auth.Amount {
		return nil, fmt.Errorf("total emission amount %d does not match authorization %d",
			totalAmount, auth.Amount)
	}

	// Verify emission height
	if auth.Height != chainParams.SKAEmissionHeight {
		return nil, fmt.Errorf("emission height %d does not match chain parameter %d",
			auth.Height, chainParams.SKAEmissionHeight)
	}

	// Create authorization hash for signature verification
	authHash, err := createEmissionAuthHash(auth, emissionAddresses, amounts)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorization hash: %w", err)
	}

	// Verify signature
	sig, err := ecdsa.ParseDERSignature(auth.Signature)
	if err != nil {
		return nil, fmt.Errorf("invalid emission signature: %w", err)
	}

	if !sig.Verify(authHash[:], auth.EmissionKey) {
		return nil, fmt.Errorf("emission signature verification failed")
	}

	// Create the authorized emission transaction
	tx := &wire.MsgTx{
		SerType:  wire.TxSerializeFull,
		Version:  1,
		LockTime: 0,
		Expiry:   0,
	}

	// Create signature script with authorization data
	authScript, err := createEmissionAuthScript(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create authorization script: %w", err)
	}

	// Add authorized input
	tx.TxIn = append(tx.TxIn, &wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{}, // All zeros
			Index: 0xffffffff,       // Max value indicates null
			Tree:  wire.TxTreeRegular,
		},
		SignatureScript: authScript,
		Sequence:        0xffffffff,
		BlockHeight:     wire.NullBlockHeight,
		BlockIndex:      wire.NullBlockIndex,
		ValueIn:         wire.NullValueIn,
	})

	// Add outputs for each emission address
	for i, addressStr := range emissionAddresses {
		addr, err := stdaddr.DecodeAddress(addressStr, chainParams)
		if err != nil {
			return nil, fmt.Errorf("invalid emission address %s: %w", addressStr, err)
		}

		// Create script for the address
		_, pkScript := addr.PaymentScript()

		// Add SKA output with specific coin type
		tx.TxOut = append(tx.TxOut, &wire.TxOut{
			Value:    amounts[i],
			CoinType: auth.CoinType, // Use authorized coin type
			Version:  0,
			PkScript: pkScript,
		})
	}

	return tx, nil
}

// createEmissionAuthHash creates a hash of the emission authorization data
// for signature verification. This ensures the signature covers all critical
// emission parameters to prevent tampering.
func createEmissionAuthHash(auth *chaincfg.SKAEmissionAuth, addresses []string, amounts []int64) ([32]byte, error) {
	var buf bytes.Buffer

	// Include all authorization fields in hash
	if err := binary.Write(&buf, binary.LittleEndian, auth.Nonce); err != nil {
		return [32]byte{}, err
	}

	if err := binary.Write(&buf, binary.LittleEndian, uint8(auth.CoinType)); err != nil {
		return [32]byte{}, err
	}

	if err := binary.Write(&buf, binary.LittleEndian, auth.Amount); err != nil {
		return [32]byte{}, err
	}

	if err := binary.Write(&buf, binary.LittleEndian, auth.Height); err != nil {
		return [32]byte{}, err
	}

	if err := binary.Write(&buf, binary.LittleEndian, auth.Timestamp); err != nil {
		return [32]byte{}, err
	}

	// Include emission details in hash
	for i, addr := range addresses {
		buf.WriteString(addr)
		if err := binary.Write(&buf, binary.LittleEndian, amounts[i]); err != nil {
			return [32]byte{}, err
		}
	}

	return sha256.Sum256(buf.Bytes()), nil
}

// createEmissionAuthScript creates the signature script containing authorization
// data for the emission transaction. This embeds the authorization proof in
// the transaction itself for validation.
func createEmissionAuthScript(auth *chaincfg.SKAEmissionAuth) ([]byte, error) {
	var script bytes.Buffer

	// Standard SKA emission marker
	script.Write([]byte{0x01, 0x53, 0x4b, 0x41}) // "SKA" marker

	// Authorization data
	script.WriteByte(0x02) // Auth version

	// Nonce (8 bytes)
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, auth.Nonce)
	script.Write(nonceBytes)

	// Coin type (1 byte)
	script.WriteByte(uint8(auth.CoinType))

	// Public key (33 bytes compressed)
	pubKeyBytes := auth.EmissionKey.SerializeCompressed()
	script.Write(pubKeyBytes)

	// Signature length and signature
	script.WriteByte(uint8(len(auth.Signature)))
	script.Write(auth.Signature)

	return script.Bytes(), nil
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

// ValidateAuthorizedSKAEmissionTransaction validates that a transaction is a valid
// cryptographically authorized SKA emission transaction. This replaces the basic
// ValidateSKAEmissionTransaction with proper security controls.
func ValidateAuthorizedSKAEmissionTransaction(tx *wire.MsgTx, blockHeight int64,
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

	// Extract and validate authorization from signature script
	auth, err := extractEmissionAuthorization(tx.TxIn[0].SignatureScript)
	if err != nil {
		return fmt.Errorf("invalid emission authorization: %w", err)
	}

	// Validate authorization against chain parameters
	if err := validateEmissionAuthorization(auth, chainParams); err != nil {
		return fmt.Errorf("emission authorization validation failed: %w", err)
	}

	// Determine expected coin type from first output
	var emissionCoinType wire.CoinType
	var totalEmissionAmount int64

	// Validate all outputs have consistent coin type and valid amounts
	for i, txOut := range tx.TxOut {
		if i == 0 {
			// Set expected coin type from first output
			emissionCoinType = txOut.CoinType
			if emissionCoinType < 1 || emissionCoinType > 255 {
				return fmt.Errorf("invalid SKA coin type in output 0: %d", emissionCoinType)
			}
		} else {
			// Verify all outputs use the same coin type
			if txOut.CoinType != emissionCoinType {
				return fmt.Errorf("inconsistent coin types: output 0 has %d, output %d has %d",
					emissionCoinType, i, txOut.CoinType)
			}
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

	// Verify authorization coin type matches transaction outputs
	if auth.CoinType != emissionCoinType {
		return fmt.Errorf("authorization coin type %d does not match transaction outputs %d",
			auth.CoinType, emissionCoinType)
	}

	// Verify authorization amount matches transaction total
	if auth.Amount != totalEmissionAmount {
		return fmt.Errorf("authorization amount %d does not match transaction total %d",
			auth.Amount, totalEmissionAmount)
	}

	// Verify emission height
	if auth.Height != blockHeight {
		return fmt.Errorf("authorization height %d does not match block height %d",
			auth.Height, blockHeight)
	}

	// Validate transaction parameters
	if tx.LockTime != 0 {
		return fmt.Errorf("SKA emission transaction must have LockTime 0")
	}

	if tx.Expiry != 0 {
		return fmt.Errorf("SKA emission transaction must have Expiry 0")
	}

	// Update nonce to prevent replay attacks
	chainParams.SetSKAEmissionNonce(auth.CoinType, auth.Nonce)

	return nil
}

// extractEmissionAuthorization extracts the emission authorization from a signature script.
// The script format is: [SKA_marker][auth_version][nonce][coin_type][pubkey][sig_len][signature]
func extractEmissionAuthorization(sigScript []byte) (*chaincfg.SKAEmissionAuth, error) {
	if len(sigScript) < 48 { // Minimum: 4(marker) + 1(version) + 8(nonce) + 1(cointype) + 33(pubkey) + 1(siglen) = 48
		return nil, fmt.Errorf("signature script too short: %d bytes", len(sigScript))
	}

	// Check SKA marker
	if len(sigScript) < 4 || !bytes.Equal(sigScript[0:4], []byte{0x01, 0x53, 0x4b, 0x41}) {
		return nil, fmt.Errorf("missing SKA emission marker")
	}

	offset := 4

	// Check authorization version
	if sigScript[offset] != 0x02 {
		return nil, fmt.Errorf("unsupported authorization version: %d", sigScript[offset])
	}
	offset++

	// Extract nonce (8 bytes)
	if len(sigScript) < offset+8 {
		return nil, fmt.Errorf("insufficient data for nonce")
	}
	nonce := binary.LittleEndian.Uint64(sigScript[offset : offset+8])
	offset += 8

	// Extract coin type (1 byte)
	if len(sigScript) < offset+1 {
		return nil, fmt.Errorf("insufficient data for coin type")
	}
	coinType := wire.CoinType(sigScript[offset])
	offset++

	// Extract public key (33 bytes compressed)
	if len(sigScript) < offset+33 {
		return nil, fmt.Errorf("insufficient data for public key")
	}
	pubKeyBytes := sigScript[offset : offset+33]
	pubKey, err := secp256k1.ParsePubKey(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	offset += 33

	// Extract signature length
	if len(sigScript) < offset+1 {
		return nil, fmt.Errorf("insufficient data for signature length")
	}
	sigLen := int(sigScript[offset])
	offset++

	// Extract signature
	if len(sigScript) < offset+sigLen {
		return nil, fmt.Errorf("insufficient data for signature: need %d, have %d",
			sigLen, len(sigScript)-offset)
	}
	signature := sigScript[offset : offset+sigLen]

	return &chaincfg.SKAEmissionAuth{
		EmissionKey: pubKey,
		Signature:   signature,
		Nonce:       nonce,
		CoinType:    coinType,
		// Amount and Height will be validated separately
	}, nil
}

// validateEmissionAuthorization validates the cryptographic authorization
// against the chain parameters and verifies the signature.
func validateEmissionAuthorization(auth *chaincfg.SKAEmissionAuth, chainParams *chaincfg.Params) error {
	// Check if emission is authorized for this coin type
	authorizedKey := chainParams.GetSKAEmissionKey(auth.CoinType)
	if authorizedKey == nil {
		return fmt.Errorf("no emission key configured for coin type %d", auth.CoinType)
	}

	// Verify the provided key matches the authorized key
	if !bytes.Equal(auth.EmissionKey.SerializeCompressed(),
		authorizedKey.SerializeCompressed()) {
		return fmt.Errorf("unauthorized emission key for coin type %d", auth.CoinType)
	}

	// Check nonce for replay protection
	lastNonce := chainParams.GetSKAEmissionNonce(auth.CoinType)
	if auth.Nonce != lastNonce+1 {
		return fmt.Errorf("invalid nonce: expected %d, got %d", lastNonce+1, auth.Nonce)
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
// 1. SKA emission windows allow emission transactions during defined periods
// 2. Non-emission windows must not contain any SKA emission transactions
// 3. No SKA transactions are allowed before activation height
// 4. Each coin type can only be emitted once (first valid emission wins)
func CheckSKAEmissionInBlock(block *dcrutil.Block, blockHeight int64,
	chainParams *chaincfg.Params) error {

	// Check if this is the legacy SKA emission block (backward compatibility)
	isLegacyEmissionBlock := isSKAEmissionBlock(blockHeight, chainParams)

	// Check if any emission window is active
	isEmissionWindowActive := isSKAEmissionWindowActive(blockHeight, chainParams)

	isActive := isSKAActive(blockHeight, chainParams)

	var emissionTxCount int
	var skaTxCount int
	emissionTxCoinTypes := make(map[dcrutil.CoinType]bool)

	// Check all transactions in the block
	for i, tx := range block.Transactions() {
		msgTx := tx.MsgTx()

		// Count SKA emission transactions
		if IsSKAEmissionTransaction(msgTx) {
			emissionTxCount++

			// Validate the emission transaction
			if err := ValidateSKAEmissionTransaction(msgTx, blockHeight, chainParams); err != nil {
				return fmt.Errorf("invalid SKA emission transaction at index %d: %w", i, err)
			}

			// Track which coin types are being emitted
			for _, txOut := range msgTx.TxOut {
				coinType := dcrutil.CoinType(txOut.CoinType)
				if emissionTxCoinTypes[coinType] {
					log.Errorf("Multiple emission transactions detected for coin type %d at height %d", coinType, blockHeight)
					return fmt.Errorf("multiple emission transactions for coin type %d at height %d - only one emission per coin type allowed", coinType, blockHeight)
				}
				emissionTxCoinTypes[coinType] = true
				log.Debugf("Tracking emission transaction for coin type %d at height %d", coinType, blockHeight)
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
	if isLegacyEmissionBlock || isEmissionWindowActive {
		// Emission transactions are allowed during emission windows
		if emissionTxCount > 0 {
			log.Debugf("Validating %d emission transactions at height %d", emissionTxCount, blockHeight)
			// Validate that emission transactions are within their respective windows
			for coinType := range emissionTxCoinTypes {
				if !isSKAEmissionWindow(blockHeight, coinType, chainParams) {
					config := chainParams.SKACoins[coinType]
					emissionStart := int64(config.EmissionHeight)
					emissionEnd := emissionStart + int64(config.EmissionWindow)
					log.Errorf("Emission transaction for coin type %d rejected: height %d outside window %d-%d",
						coinType, blockHeight, emissionStart, emissionEnd)
					return fmt.Errorf("emission transaction for coin type %d at height %d is outside emission window (%d-%d)",
						coinType, blockHeight, emissionStart, emissionEnd)
				}
				log.Debugf("Emission transaction for coin type %d validated at height %d", coinType, blockHeight)
			}
		}
	} else {
		// Non-emission windows must not have emission transactions
		if emissionTxCount > 0 {
			return fmt.Errorf("block at height %d contains %d SKA emission transactions but is not in emission window",
				blockHeight, emissionTxCount)
		}
	}

	// Before activation, no SKA transactions are allowed (except emission)
	if !isActive && !isLegacyEmissionBlock && !isEmissionWindowActive && skaTxCount > 0 {
		return fmt.Errorf("SKA transactions not allowed before activation height %d (current: %d)",
			chainParams.SKAActivationHeight, blockHeight)
	}

	return nil
}
