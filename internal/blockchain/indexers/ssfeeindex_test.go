// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"bytes"
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/txscript"
	"github.com/monetarium/monetarium-node/wire"
)

// TestSSFeeIndexKey tests the makeSSFeeIndexKey function with various inputs.
func TestSSFeeIndexKey(t *testing.T) {
	tests := []struct {
		name      string
		feeType   byte
		coinType  cointype.CoinType
		hash160   []byte
		wantLen   int
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid staker VAR coin type",
			feeType:  ssfeeTypeStaker,
			coinType: cointype.CoinTypeVAR,
			hash160:  make([]byte, 20),
			wantLen:  24,
			wantErr:  false,
		},
		{
			name:     "valid miner VAR coin type",
			feeType:  ssfeeTypeMiner,
			coinType: cointype.CoinTypeVAR,
			hash160:  make([]byte, 20),
			wantLen:  24,
			wantErr:  false,
		},
		{
			name:     "valid staker SKA-1 coin type",
			feeType:  ssfeeTypeStaker,
			coinType: cointype.CoinType(1),
			hash160:  make([]byte, 20),
			wantLen:  24,
			wantErr:  false,
		},
		{
			name:     "valid miner SKA-2 coin type",
			feeType:  ssfeeTypeMiner,
			coinType: cointype.CoinType(2),
			hash160:  make([]byte, 20),
			wantLen:  24,
			wantErr:  false,
		},
		{
			name:      "invalid hash160 length (too short)",
			feeType:   ssfeeTypeStaker,
			coinType:  cointype.CoinTypeVAR,
			hash160:   make([]byte, 19),
			wantLen:   0,
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
		{
			name:      "invalid hash160 length (too long)",
			feeType:   ssfeeTypeStaker,
			coinType:  cointype.CoinTypeVAR,
			hash160:   make([]byte, 21),
			wantLen:   0,
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
		{
			name:      "nil hash160",
			feeType:   ssfeeTypeStaker,
			coinType:  cointype.CoinTypeVAR,
			hash160:   nil,
			wantLen:   0,
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := makeSSFeeIndexKey(tt.feeType, tt.coinType, tt.hash160)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errSubstr)) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(key) != tt.wantLen {
				t.Fatalf("expected key length %d, got %d", tt.wantLen, len(key))
			}

			// Verify key format: "sf" + feeType + coinType + hash160
			if !bytes.Equal(key[0:2], []byte("sf")) {
				t.Fatalf("expected prefix 'sf', got %q", key[0:2])
			}

			if key[2] != tt.feeType {
				t.Fatalf("expected feeType %d, got %d", tt.feeType, key[2])
			}

			if key[3] != byte(tt.coinType) {
				t.Fatalf("expected coinType %d, got %d", tt.coinType, key[3])
			}

			if !bytes.Equal(key[4:24], tt.hash160) {
				t.Fatalf("expected hash160 %x, got %x", tt.hash160, key[4:24])
			}
		})
	}
}

// TestExtractHash160FromPkScript tests the extractHash160FromPkScript function.
func TestExtractHash160FromPkScript(t *testing.T) {
	// Create a valid P2PKH script
	hash160 := []byte{
		0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
		0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x9a, 0x0b,
	}

	validP2PKH := make([]byte, 25)
	validP2PKH[0] = txscript.OP_DUP
	validP2PKH[1] = txscript.OP_HASH160
	validP2PKH[2] = txscript.OP_DATA_20
	copy(validP2PKH[3:23], hash160)
	validP2PKH[23] = txscript.OP_EQUALVERIFY
	validP2PKH[24] = txscript.OP_CHECKSIG

	tests := []struct {
		name      string
		pkScript  []byte
		want      []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name:     "valid P2PKH script",
			pkScript: validP2PKH,
			want:     hash160,
			wantErr:  false,
		},
		{
			name:      "script too short",
			pkScript:  make([]byte, 24),
			want:      nil,
			wantErr:   true,
			errSubstr: "invalid P2PKH script length",
		},
		{
			name:      "script too long",
			pkScript:  make([]byte, 27),
			want:      nil,
			wantErr:   true,
			errSubstr: "invalid P2PKH script length",
		},
		{
			name: "invalid opcode sequence",
			pkScript: []byte{
				txscript.OP_DUP, txscript.OP_HASH160, txscript.OP_DATA_20,
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
				txscript.OP_EQUAL, // Wrong opcode (should be EQUALVERIFY)
				txscript.OP_CHECKSIG,
			},
			want:      nil,
			wantErr:   true,
			errSubstr: "not a valid P2PKH script",
		},
		{
			name:      "nil script",
			pkScript:  nil,
			want:      nil,
			wantErr:   true,
			errSubstr: "invalid P2PKH script length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractHash160FromPkScript(tt.pkScript)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errSubstr)) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(got, tt.want) {
				t.Fatalf("expected hash160 %x, got %x", tt.want, got)
			}
		})
	}
}

// TestSerializeDeserializeOutPoints tests outpoint serialization and deserialization.
func TestSerializeDeserializeOutPoints(t *testing.T) {
	tests := []struct {
		name      string
		outpoints []wire.OutPoint
	}{
		{
			name:      "empty list",
			outpoints: []wire.OutPoint{},
		},
		{
			name: "single outpoint",
			outpoints: []wire.OutPoint{
				{
					Hash:  *newHashFromStr("000000000000000000000000000000000000000000000000000000000000000a"),
					Index: 0,
					Tree:  wire.TxTreeStake,
				},
			},
		},
		{
			name: "multiple outpoints",
			outpoints: []wire.OutPoint{
				{
					Hash:  *newHashFromStr("000000000000000000000000000000000000000000000000000000000000000a"),
					Index: 0,
					Tree:  wire.TxTreeStake,
				},
				{
					Hash:  *newHashFromStr("000000000000000000000000000000000000000000000000000000000000000b"),
					Index: 1,
					Tree:  wire.TxTreeStake,
				},
				{
					Hash:  *newHashFromStr("000000000000000000000000000000000000000000000000000000000000000c"),
					Index: 2,
					Tree:  wire.TxTreeStake,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			serialized := serializeOutPoints(tt.outpoints)

			// Check serialized length
			expectedLen := len(tt.outpoints) * outpointSize
			if len(serialized) != expectedLen {
				t.Fatalf("expected serialized length %d, got %d", expectedLen, len(serialized))
			}

			// Deserialize
			deserialized, err := deserializeOutPoints(serialized)
			if err != nil {
				t.Fatalf("deserialization failed: %v", err)
			}

			// Check deserialized length
			if len(deserialized) != len(tt.outpoints) {
				t.Fatalf("expected %d outpoints, got %d", len(tt.outpoints), len(deserialized))
			}

			// Verify each outpoint
			for i, want := range tt.outpoints {
				got := deserialized[i]
				if got.Hash != want.Hash {
					t.Fatalf("outpoint[%d]: expected hash %v, got %v", i, want.Hash, got.Hash)
				}
				if got.Index != want.Index {
					t.Fatalf("outpoint[%d]: expected index %d, got %d", i, want.Index, got.Index)
				}
				if got.Tree != want.Tree {
					t.Fatalf("outpoint[%d]: expected tree %d, got %d", i, want.Tree, got.Tree)
				}
			}
		})
	}
}

// TestDeserializeOutPointsInvalid tests deserialization error handling.
func TestDeserializeOutPointsInvalid(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "invalid length (not multiple of 37)",
			data:      make([]byte, 36),
			wantErr:   true,
			errSubstr: "invalid outpoint data length",
		},
		{
			name:      "invalid length (partial outpoint)",
			data:      make([]byte, 50),
			wantErr:   true,
			errSubstr: "invalid outpoint data length",
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outpoints, err := deserializeOutPoints(tt.data)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errSubstr)) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(outpoints) != 0 {
				t.Fatalf("expected empty outpoint list, got %d outpoints", len(outpoints))
			}
		})
	}
}

// TestSSFeeIndexKeyUniqueness tests that different (feeType, coinType, address) tuples
// produce unique keys.
func TestSSFeeIndexKeyUniqueness(t *testing.T) {
	hash160_1 := make([]byte, 20)
	hash160_2 := make([]byte, 20)
	hash160_2[0] = 0x01 // Make it different

	keys := make(map[string]bool)

	testCases := []struct {
		feeType  byte
		coinType cointype.CoinType
		hash160  []byte
	}{
		{ssfeeTypeStaker, cointype.CoinTypeVAR, hash160_1},
		{ssfeeTypeStaker, cointype.CoinTypeVAR, hash160_2},
		{ssfeeTypeMiner, cointype.CoinTypeVAR, hash160_1},
		{ssfeeTypeMiner, cointype.CoinTypeVAR, hash160_2},
		{ssfeeTypeStaker, cointype.CoinType(1), hash160_1},
		{ssfeeTypeStaker, cointype.CoinType(1), hash160_2},
		{ssfeeTypeMiner, cointype.CoinType(1), hash160_1},
		{ssfeeTypeMiner, cointype.CoinType(1), hash160_2},
		{ssfeeTypeStaker, cointype.CoinType(2), hash160_1},
		{ssfeeTypeStaker, cointype.CoinType(2), hash160_2},
		{ssfeeTypeMiner, cointype.CoinType(2), hash160_1},
		{ssfeeTypeMiner, cointype.CoinType(2), hash160_2},
	}

	for _, tc := range testCases {
		key, err := makeSSFeeIndexKey(tc.feeType, tc.coinType, tc.hash160)
		if err != nil {
			t.Fatalf("unexpected error creating key: %v", err)
		}

		keyStr := string(key)
		if keys[keyStr] {
			t.Fatalf("duplicate key generated for feeType=%d, coinType=%d, hash160=%x",
				tc.feeType, tc.coinType, tc.hash160)
		}
		keys[keyStr] = true
	}

	// Verify we have all unique keys
	if len(keys) != len(testCases) {
		t.Fatalf("expected %d unique keys, got %d", len(testCases), len(keys))
	}
}

// newHashFromStr converts a hex string to a chainhash.Hash.
// Panics if the string is not a valid hash.
func newHashFromStr(hexStr string) *chainhash.Hash {
	hash, err := chainhash.NewHashFromStr(hexStr)
	if err != nil {
		panic(err)
	}
	return hash
}
