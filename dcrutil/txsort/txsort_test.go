// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2017-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txsort

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/monetarium/monetarium-node/wire"
)

// TestSort ensures the transaction sorting works as expected.
func TestSort(t *testing.T) {
	tests := []struct {
		name         string
		hexFile      string
		isSorted     bool
		unsortedHash string
		sortedHash   string
	}{
		{
			name:         "block 100004 tx[4] - already sorted",
			hexFile:      "tx100004-4.hex",
			isSorted:     true,
			unsortedHash: "2d7f7fc1c7c8a4ef1b331d96daa8af7c468ca32c153c681f2f7b1b5e74b3014c",
			sortedHash:   "2d7f7fc1c7c8a4ef1b331d96daa8af7c468ca32c153c681f2f7b1b5e74b3014c",
		},
		{
			name:         "block 101790 tx[3] - sorts inputs only, based on tree",
			hexFile:      "tx101790-3.hex",
			isSorted:     false,
			unsortedHash: "05bcfb356cafedb3a1ced7b86c0eb1899954d1062d216b9c6bde19facc78b6b4",
			sortedHash:   "e0b4dc0d6b37777e584edaf30db8382d41d337d3c12e8ccbe030f06ddce3ca95",
		},
		{
			name:         "block 150007 tx[23] - sorts inputs only, based on hash",
			hexFile:      "tx150007-23.hex",
			isSorted:     false,
			unsortedHash: "ca0c29fa943b2ac629d17c66ef59bd866b5f39c8c22abf99c351e998fec62e61",
			sortedHash:   "8da32f59fc9f8c2f941ed2fdf0c16525c02e6caa71211204b10c0a93128dc672",
		},
		{
			name:         "block 108930 tx[1] - sorts inputs only, based on index",
			hexFile:      "tx108930-1.hex",
			isSorted:     false,
			unsortedHash: "d7c3f51f1e45262fb59aa5d2590703715c6fe947ec543538621135d8140836ae",
			sortedHash:   "c1ff28f4376da33c269ad692eec220068dc28aeb4860fb886317f44fdd1bfc1e",
		},
		{
			name:         "block 100082 tx[5] - sorts outputs only, based on amount",
			hexFile:      "tx100082-5.hex",
			isSorted:     false,
			unsortedHash: "b97731f43deefe91bf02e1fe046dc3f6d2ae7d08186d88494112782a443b14ce",
			sortedHash:   "f7304d057fff93479229c9c856306b49c92bf2aee30ccd79722e14e3504374dc",
		},
		{
			// Tx manually modified to make the first output (output 0)
			// have script version 1.
			name:         "modified block 150043 tx[14] - sorts outputs only, based on script version",
			hexFile:      "tx150043-14m.hex",
			isSorted:     false,
			unsortedHash: "b69ca33df9a7e32defba0ce31612fb33e74a1461e83d3d08df1272ac788c8fc8",
			sortedHash:   "79bec563b49c5fc955f89e959a8c571846db3e50bc66e7edf6880a5024a9d175",
		},
		{
			name:         "block 150043 tx[14] - sorts outputs only, based on output script",
			hexFile:      "tx150043-14.hex",
			isSorted:     false,
			unsortedHash: "e34662435095f0803ca358cd233e7ac2567b6c9c7bc3469b2cc6074e45b85e0b",
			sortedHash:   "43d4962b0bfaadf1737dcf930090c0ce85750972cf87fbcbfc7da3d8620a5f41",
		},
		{
			name:         "block 150626 tx[24] - sorts outputs only, based on amount and output script",
			hexFile:      "tx150626-24.hex",
			isSorted:     false,
			unsortedHash: "f9da9ef41f5790fab6cf2930cdfd2d6d8e3a346cc8279650d71b40a664a224ce",
			sortedHash:   "8d195aeb1ad2fd9cd6054e9b23c0a0f2a9fb170feb6519d3a749bf0725e00118",
		},
		{
			name:         "block 150002 tx[7] - sorts both inputs and outputs",
			hexFile:      "tx150002-7.hex",
			isSorted:     false,
			unsortedHash: "83e3ee1b1f207361577deda6efcb626a65fe2b7cd34c451bf7de9e7c36f0f717",
			sortedHash:   "5137f03a0e35c16e68de2f820edaa60262bcc083f44936b0139b05e31c0354aa",
		},
	}

	for _, test := range tests {
		// Load and deserialize the test transaction.
		filePath := filepath.Join("testdata", test.hexFile)
		txHexBytes, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("ReadFile (%s): failed to read test file: %v",
				test.name, err)
			continue
		}
		txBytes, err := hex.DecodeString(string(txHexBytes))
		if err != nil {
			t.Errorf("DecodeString (%s): failed to decode tx: %v",
				test.name, err)
			continue
		}
		var tx wire.MsgTx
		err = tx.Deserialize(bytes.NewReader(txBytes))
		if err != nil {
			t.Errorf("Deserialize (%s): unexpected error %v",
				test.name, err)
			continue
		}

		// Ensure the sort order of the original transaction matches the
		// expected value.
		if got := IsSorted(&tx); got != test.isSorted {
			t.Errorf("IsSorted (%s): sort does not match "+
				"expected - got %v, want %v", test.name, got,
				test.isSorted)
			continue
		}

		// Sort the transaction and ensure the resulting hash is the
		// expected value.
		sortedTx := Sort(&tx)
		if got := sortedTx.TxHash().String(); got != test.sortedHash {
			t.Errorf("Sort (%s): sorted hash does not match "+
				"expected - got %v, want %v", test.name, got,
				test.sortedHash)
			continue
		}

		// Ensure the original transaction is not modified.
		if got := tx.TxHash().String(); got != test.unsortedHash {
			t.Errorf("Sort (%s): unsorted hash does not match "+
				"expected - got %v, want %v", test.name, got,
				test.unsortedHash)
			continue
		}

		// Now sort the transaction using the mutable version and ensure
		// the resulting hash is the expected value.
		InPlaceSort(&tx)
		if got := tx.TxHash().String(); got != test.sortedHash {
			t.Errorf("SortMutate (%s): sorted hash does not match "+
				"expected - got %v, want %v", test.name, got,
				test.sortedHash)
			continue
		}
	}
}
