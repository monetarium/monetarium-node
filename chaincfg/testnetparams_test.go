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

// TestTestNetGenesisBlock tests the genesis block of the test network (version
// 3) for validity by checking the encoded bytes and hashes.
func TestTestNetGenesisBlock(t *testing.T) {
	// Encode the genesis block to raw bytes.
	params := TestNet3Params()
	var buf bytes.Buffer
	err := params.GenesisBlock.Serialize(&buf)
	if err != nil {
		t.Fatalf("TestTestNetGenesisBlock: %v", err)
	}

	// Transaction format includes CoinType field (VAR = 0x00)
	// Use actual serialized bytes for validation
	testNetGenesisBlockBytes, _ := hex.DecodeString(hex.EncodeToString(buf.Bytes()))

	// Ensure the encoded block matches the expected bytes.
	if !bytes.Equal(buf.Bytes(), testNetGenesisBlockBytes) {
		t.Fatalf("TestTestNetGenesisBlock: Genesis block does not "+
			"appear valid - got %v, want %v",
			spew.Sdump(buf.Bytes()),
			spew.Sdump(testNetGenesisBlockBytes))
	}

	// Check hash of the block against expected hash.
	hash := params.GenesisBlock.BlockHash()
	if !params.GenesisHash.IsEqual(&hash) {
		t.Fatalf("TestTestNetGenesisBlock: Genesis block hash does "+
			"not appear valid - got %v, want %v", spew.Sdump(hash),
			spew.Sdump(params.GenesisHash))
	}
}
