// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2015-2022 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package standalone

import (
	"fmt"
	"math"
	"math/big"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

const (
	// These constants are opcodes are defined here to avoid a dependency on
	// txscript.  They are used in consensus code which can't be changed without
	// a vote anyway, so not referring to them directly via txscript is safe.
	opData12 = 0x0c
	opReturn = 0x6a
	opTAdd   = 0xc1
	opTSpend = 0xc2
	opTGen   = 0xc3

	// These constants use values from cointype package for consistency.
	// They are used in consensus code which can't be changed without a vote anyway.
	//
	// atomsPerCoin is the number of atoms in one coin.
	//
	// maxAtoms is the maximum transaction amount allowed in atoms.
	atomsPerCoin = cointype.AtomsPerVAR
	maxAtoms     = cointype.MaxVARAtoms

	// NOTE: SKA amounts use big.Int (via TxOut.SKAValue) because 900 trillion
	// coins with 1e18 atoms/coin exceeds int64 capacity. Max supply validation
	// for SKA happens in context-aware code with chain params.
)

var (
	// zeroHash is the zero value for a chainhash.Hash and is defined as a
	// package level variable to avoid the need to create a new instance every
	// time a check is needed.
	zeroHash = chainhash.Hash{}
)

// isValidCoinType checks if the given coin type is within valid range.
// Note: This only validates range (0-255). For activation-aware validation,
// use the cointype package with chain parameters.
func isValidCoinType(_ cointype.CoinType) bool {
	// Valid range is 0-255 (enforced by uint8, but explicit for clarity)
	return true // All uint8 values (0-255) are valid coin type numbers
}

// getMaxAtomsForCoinType returns the maximum atoms allowed for a given coin type.
// For VAR, returns maxAtoms. For SKA, returns 0 (SKA uses big.Int, not int64).
func getMaxAtomsForCoinType(coinType cointype.CoinType) int64 {
	switch coinType {
	case cointype.CoinTypeVAR:
		return maxAtoms
	default:
		// SKA uses big.Int for amounts. Return 0 to indicate int64 is not applicable.
		// Callers should use TxOut.SKAValue (*big.Int) for SKA amounts.
		return 0
	}
}

// IsCoinBaseTx determines whether or not a transaction is a coinbase.  A
// coinbase is a special transaction created by miners that has no inputs.
// This is represented in the block chain by a transaction with a single input
// that has a previous output transaction index set to the maximum value along
// with a zero hash.
func IsCoinBaseTx(tx *wire.MsgTx, isTreasuryEnabled bool) bool {
	// A coinbase must be version 3 once the treasury agenda is active.
	if isTreasuryEnabled && tx.Version != wire.TxVersionTreasury {
		return false
	}

	// A coinbase must only have one transaction input.
	if len(tx.TxIn) != 1 {
		return false
	}

	// The previous output of a coinbase must have a max value index and a
	// zero hash.
	prevOut := &tx.TxIn[0].PreviousOutPoint
	if prevOut.Index != math.MaxUint32 || prevOut.Hash != zeroHash {
		return false
	}

	// isTreasurySpendLike returns whether or not the provided transaction is
	// likely to be a treasury spend transaction for the purposes of
	// differentiating it from a coinbase.
	//
	// Note that this relies on the checks above to avoid panics.
	isTreasurySpendLike := func(tx *wire.MsgTx) bool {
		// Treasury spends have at least two outputs.
		if len(tx.TxOut) < 2 {
			return false
		}

		// Treasury spends have scripts in the first input and all outputs.
		l := len(tx.TxIn[0].SignatureScript)
		if l == 0 ||
			len(tx.TxOut[0].PkScript) == 0 ||
			len(tx.TxOut[1].PkScript) == 0 {

			return false
		}

		// Treasury spends have an OP_TSPEND as the last byte of the signature
		// script of the first input, an OP_RETURN as the first byte of the
		// public key script of the first output, and at least one output with
		// an OP_TGEN as the first byte of its public key script.
		return tx.TxIn[0].SignatureScript[l-1] == opTSpend &&
			tx.TxOut[0].PkScript[0] == opReturn &&
			tx.TxOut[1].PkScript[0] == opTGen
	}

	// Avoid detecting treasury spends as a coinbase transaction when the
	// treasury agenda is active.
	if isTreasuryEnabled && isTreasurySpendLike(tx) {
		return false
	}

	// Avoid detecting SSFee transactions as coinbase.
	// SSFee transactions have an OP_RETURN output with "SF" or "MF" marker bytes.
	if tx.Version >= 3 && len(tx.TxOut) >= 2 && len(tx.TxIn[0].SignatureScript) == 0 {
		// Check for SSFee marker in any OP_RETURN output
		for _, out := range tx.TxOut {
			if IsSSFeeMarkerScript(out.PkScript) {
				return false // This is SSFee, not coinbase
			}
		}
	}

	// Avoid detecting SKA emission transactions as coinbase.
	// SKA emissions have a signature script starting with [0x01][0x53][0x4B][0x41] marker.
	if len(tx.TxIn[0].SignatureScript) >= 4 {
		sigScript := tx.TxIn[0].SignatureScript
		// SKA marker: 0x01 (length) + "SKA" (0x53 0x4B 0x41)
		if sigScript[0] == 0x01 && sigScript[1] == 0x53 && sigScript[2] == 0x4B && sigScript[3] == 0x41 {
			return false // This is SKA emission, not coinbase
		}
	}

	return true
}

// isNullOutpoint determines whether or not a previous transaction output point
// is set.
func isNullOutpoint(tx *wire.MsgTx) bool {
	nullInOP := tx.TxIn[0].PreviousOutPoint
	if nullInOP.Index == math.MaxUint32 && nullInOP.Hash.IsEqual(&zeroHash) &&
		nullInOP.Tree == wire.TxTreeRegular {
		return true
	}
	return false
}

// IsTreasuryBase does a minimal check to see if a transaction is a treasury
// base.
func IsTreasuryBase(tx *wire.MsgTx) bool {
	if tx.Version != wire.TxVersionTreasury {
		return false
	}

	if len(tx.TxIn) != 1 || len(tx.TxOut) != 2 {
		return false
	}

	if len(tx.TxIn[0].SignatureScript) != 0 {
		return false
	}

	if len(tx.TxOut[0].PkScript) != 1 || tx.TxOut[0].PkScript[0] != opTAdd {
		return false
	}

	if len(tx.TxOut[1].PkScript) != 14 ||
		tx.TxOut[1].PkScript[0] != opReturn ||
		tx.TxOut[1].PkScript[1] != opData12 {

		return false
	}

	return isNullOutpoint(tx)
}

// CheckTransactionSanity performs some preliminary checks on a transaction to
// ensure it is sane.  These checks are context free.
func CheckTransactionSanity(tx *wire.MsgTx, maxTxSize uint64) error {
	// A transaction must have at least one input.
	if len(tx.TxIn) == 0 {
		return ruleError(ErrNoTxInputs, "transaction has no inputs")
	}

	// A transaction must have at least one output.
	if len(tx.TxOut) == 0 {
		return ruleError(ErrNoTxOutputs, "transaction has no outputs")
	}

	// A transaction must not exceed the maximum allowed size when serialized.
	serializedTxSize := uint64(tx.SerializeSize())
	if serializedTxSize > maxTxSize {
		str := fmt.Sprintf("serialized transaction is too big - got %d, max %d",
			serializedTxSize, maxTxSize)
		return ruleError(ErrTxTooBig, str)
	}

	// Ensure the transaction amounts are in range.  Each transaction output
	// must not be negative or more than the max allowed per transaction.  Also,
	// the total of all outputs must abide by the same restrictions.  All
	// amounts in a transaction are in a unit value known as an atom.  One
	// coin is a quantity of atoms as defined by the AtomsPerCoin constant.
	var totalVARAtoms int64
	totalSKAAtoms := new(big.Int) // SKA uses big.Int for amounts up to 900T * 1e18
	zeroBI := new(big.Int)

	for i, txOut := range tx.TxOut {
		coinType := cointype.CoinType(uint8(txOut.CoinType))

		// Validate coin type
		if !isValidCoinType(coinType) {
			str := fmt.Sprintf("transaction output has invalid coin type %d", coinType)
			return ruleError(ErrBadTxOutValue, str)
		}

		// Validate that the correct value field is used for the coin type.
		// This prevents ambiguous outputs that could be exploited.
		// VAR: must use Value (int64), SKAValue must be nil
		// SKA: must use SKAValue (big.Int), Value must be 0
		if err := txOut.ValidateCoinTypeFields(); err != nil {
			str := fmt.Sprintf("transaction output %d: %v", i, err)
			return ruleError(ErrBadTxOutValue, str)
		}

		// Track totals by coin type
		switch coinType {
		case cointype.CoinTypeVAR:
			atoms := txOut.Value

			// Validate amount is non-negative
			if atoms < 0 {
				str := fmt.Sprintf("transaction output has negative value of %v", atoms)
				return ruleError(ErrBadTxOutValue, str)
			}

			// Check against VAR max
			if atoms > maxAtoms {
				str := fmt.Sprintf("transaction output value of %v is higher than "+
					"max allowed value of %v for VAR", atoms, maxAtoms)
				return ruleError(ErrBadTxOutValue, str)
			}

			// Two's complement int64 overflow guarantees that any overflow is
			// detected and reported.
			totalVARAtoms += atoms
			if totalVARAtoms < 0 {
				str := fmt.Sprintf("total value of all VAR transaction outputs "+
					"exceeds max allowed value of %v", maxAtoms)
				return ruleError(ErrBadTxOutValue, str)
			}
			if totalVARAtoms > maxAtoms {
				str := fmt.Sprintf("total value of all VAR transaction outputs is %v "+
					"which is higher than max allowed value of %v", totalVARAtoms,
					maxAtoms)
				return ruleError(ErrBadTxOutValue, str)
			}
		default:
			// All SKA coin types (1-255) use big.Int for amounts.
			// SKAValue is guaranteed to be non-nil by ValidateCoinTypeFields above.
			skaAmount := txOut.SKAValue

			// Validate amount is non-negative
			if skaAmount.Sign() < 0 {
				str := fmt.Sprintf("transaction output has negative SKA value of %v", skaAmount)
				return ruleError(ErrBadTxOutValue, str)
			}

			// Note: Per-SKA-type max supply validation requires chain params
			// and happens in context-aware validation code.

			// Track total SKA atoms
			totalSKAAtoms.Add(totalSKAAtoms, skaAmount)

			// Check for negative total (shouldn't happen with non-negative inputs)
			if totalSKAAtoms.Cmp(zeroBI) < 0 {
				str := "total value of all SKA transaction outputs is negative"
				return ruleError(ErrBadTxOutValue, str)
			}
		}
	}

	// Check for duplicate transaction inputs.
	existingTxOut := make(map[wire.OutPoint]struct{})
	for _, txIn := range tx.TxIn {
		if _, exists := existingTxOut[txIn.PreviousOutPoint]; exists {
			str := "transaction contains duplicate inputs"
			return ruleError(ErrDuplicateTxInputs, str)
		}
		existingTxOut[txIn.PreviousOutPoint] = struct{}{}
	}

	return nil
}
