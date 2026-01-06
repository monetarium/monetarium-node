// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/txscript"
)

// TestMakeDistributionKey tests the makeDistributionKey helper function.
func TestMakeDistributionKey(t *testing.T) {
	tests := []struct {
		name        string
		coinType    cointype.CoinType
		hash160Hex  string
		expectedKey string
	}{
		{
			name:        "VAR coin type",
			coinType:    cointype.CoinTypeVAR,
			hash160Hex:  "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b",
			expectedKey: "0_1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b",
		},
		{
			name:        "SKA-1 coin type",
			coinType:    cointype.CoinType(1),
			hash160Hex:  "abcdef1234567890abcdef1234567890abcdef12",
			expectedKey: "1_abcdef1234567890abcdef1234567890abcdef12",
		},
		{
			name:        "SKA-2 coin type",
			coinType:    cointype.CoinType(2),
			hash160Hex:  "0000000000000000000000000000000000000000",
			expectedKey: "2_0000000000000000000000000000000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := makeDistributionKey(tc.coinType, tc.hash160Hex)
			if result != tc.expectedKey {
				t.Errorf("Expected key %s, got %s", tc.expectedKey, result)
			}
		})
	}
}

// TestExtractHash160FromPkScript tests hash160 extraction from different script types.
func TestExtractHash160FromPkScript(t *testing.T) {
	// Create a P2PKH script
	hash160 := []byte{
		0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
		0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x9a, 0x0b,
	}

	// P2PKH: OP_DUP OP_HASH160 OP_DATA_20 <hash160> OP_EQUALVERIFY OP_CHECKSIG
	p2pkhScript := make([]byte, 25)
	p2pkhScript[0] = txscript.OP_DUP
	p2pkhScript[1] = txscript.OP_HASH160
	p2pkhScript[2] = 0x14 // OP_DATA_20
	copy(p2pkhScript[3:23], hash160)
	p2pkhScript[23] = txscript.OP_EQUALVERIFY
	p2pkhScript[24] = txscript.OP_CHECKSIG

	// P2SH: OP_HASH160 OP_DATA_20 <hash160> OP_EQUAL
	p2shScript := make([]byte, 23)
	p2shScript[0] = txscript.OP_HASH160
	p2shScript[1] = 0x14 // OP_DATA_20
	copy(p2shScript[2:22], hash160)
	p2shScript[22] = txscript.OP_EQUAL

	tests := []struct {
		name    string
		script  []byte
		wantErr bool
	}{
		{
			name:    "Valid P2PKH script",
			script:  p2pkhScript,
			wantErr: false,
		},
		{
			name:    "Valid P2SH script",
			script:  p2shScript,
			wantErr: false,
		},
		{
			name:    "Invalid script (too short)",
			script:  []byte{0x76, 0xa9},
			wantErr: true,
		},
		{
			name:    "Invalid script (wrong opcodes)",
			script:  make([]byte, 25),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := extractHash160FromPkScript(tc.script)
			if tc.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(result) != 20 {
					t.Errorf("Expected hash160 length 20, got %d", len(result))
				}
				// For valid scripts, verify the extracted hash160 matches
				for i := 0; i < 20; i++ {
					if result[i] != hash160[i] {
						t.Errorf("Hash160 mismatch at byte %d: expected %x, got %x",
							i, hash160[i], result[i])
						break
					}
				}
			}
		})
	}
}
