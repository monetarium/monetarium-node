// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2015-2019 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

// TestGenesisBlock tests the genesis block of the main network for validity by
// checking the encoded bytes and hashes.
func TestGenesisBlock(t *testing.T) {
	// Updated genesis block bytes with new timestamp (Oct 16, 2025 = 0x68f16180)
	// and CPU-friendly difficulty (0x1d00ffff) for bootstrap
	// Transaction format includes CoinType field (VAR = 0x00)
	genesisBlockBytes, _ := hex.DecodeString("010000000000000000000000000000000000000000000000000000000000000000000000f1adbc0fe2cebd070208a804f4c8e98ed464a4fd07b55d18164552d84be12165000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ffff001d00c2eb0b0000000000000000000000008061f168000000000000000000000000000000000000000000000000000000000000000000000000000000000101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff00ffffffff01000000000000000000000020801679e98561ada96caec2949a5d41c4cab3851eb740d951c10ecbcf265c1fd90000000000000000010000000000000000000000000000000002000000")

	// Encode the genesis block to raw bytes.
	params := MainNetParams()
	var buf bytes.Buffer
	err := params.GenesisBlock.Serialize(&buf)
	if err != nil {
		t.Fatalf("TestGenesisBlock: %v", err)
	}

	// Ensure the encoded block matches the expected bytes.
	if !bytes.Equal(buf.Bytes(), genesisBlockBytes) {
		t.Fatalf("TestGenesisBlock: Genesis block does not appear valid - "+
			"got %v, want %v", spew.Sdump(buf.Bytes()),
			spew.Sdump(genesisBlockBytes))
	}

	// Check hash of the block against expected hash.
	hash := params.GenesisBlock.BlockHash()
	if !params.GenesisHash.IsEqual(&hash) {
		t.Fatalf("TestGenesisBlock: Genesis block hash does not "+
			"appear valid - got %v, want %v", spew.Sdump(hash),
			spew.Sdump(params.GenesisHash))
	}
}
