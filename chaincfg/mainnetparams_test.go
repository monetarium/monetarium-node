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
	// Changed from old timestamp (Feb 8, 2016 = 0x56b8d7a0) at offset 0x80
	genesisBlockBytes, _ := hex.DecodeString("010000000000000000000000000000000000000000000000000000000000000000000000c94a865de331f112fef22ebdf9e66ea8e5795b73e5c0c8294ff37b3b74285e7a000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000ffff001d00c2eb0b0000000000000000000000008061f168000000000000000000000000000000000000000000000000000000000000000000000000000000000101000000010000000000000000000000000000000000000000000000000000000000000000ffffffff00ffffffff01000000000000000000000020801679e98561ada96caec2949a5d41c4cab3851eb740d951c10ecbcf265c1fd9000000000000000001ffffffffffffffff00000000ffffffff02000000")

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
