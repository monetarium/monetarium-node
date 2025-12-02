// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package standalone

// SSFee marker detection utilities for the standalone package.
// This provides minimal marker detection without dependencies.
// Full SSFee utilities (including marker creation) are in blockchain/stake/ssfee.go
//
// These constants are exported so they can be reused by the stake package
// to avoid duplication.

const (
	// SSFeeOpReturn is the OP_RETURN opcode for SSFee markers.
	SSFeeOpReturn = 0x6a

	// SSFeeOpData6 is OP_DATA_6 used for miner SSFee (6 bytes data).
	SSFeeOpData6 = 0x06

	// SSFeeOpData8 is OP_DATA_8 used for staker SSFee (8 bytes data).
	SSFeeOpData8 = 0x08

	// SSFeeMarkerS is the 'S' byte in "SF" and "MF" markers.
	SSFeeMarkerS = 0x53

	// SSFeeMarkerF is the 'F' byte in "SF" and "MF" markers.
	SSFeeMarkerF = 0x46

	// SSFeeMarkerM is the 'M' byte in "MF" (miner) markers.
	SSFeeMarkerM = 0x4D

	// SSFeeMinScriptLen is the minimum length for SSFee OP_RETURN scripts.
	// OP_RETURN(1) + OP_DATA_6/8(1) + marker(2) + height(4) = 8 bytes minimum.
	SSFeeMinScriptLen = 8
)

// IsSSFeeMarkerScript checks if a script is a valid SSFee OP_RETURN marker.
// This is used by IsCoinBaseTx to distinguish SSFee from coinbase transactions.
//
// SSFee OP_RETURN format:
//   - Staker: OP_RETURN + OP_DATA_8 + "SF" + height(4 bytes) + voter_seq(2 bytes)
//   - Miner:  OP_RETURN + OP_DATA_6 + "MF" + height(4 bytes)
func IsSSFeeMarkerScript(script []byte) bool {
	// Check minimum length
	if len(script) < SSFeeMinScriptLen {
		return false
	}

	// Check for OP_RETURN
	if script[0] != SSFeeOpReturn {
		return false
	}

	// Check for OP_DATA_6 (miner) or OP_DATA_8 (staker)
	if script[1] != SSFeeOpData6 && script[1] != SSFeeOpData8 {
		return false
	}

	// Check for "SF" (Stake Fee) or "MF" (Miner Fee) marker
	if (script[2] == SSFeeMarkerS && script[3] == SSFeeMarkerF) ||
		(script[2] == SSFeeMarkerM && script[3] == SSFeeMarkerF) {
		return true
	}

	return false
}
