//go:build ignore

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monetarium/monetarium-node/dcrutil/txsort"
	"github.com/monetarium/monetarium-node/wire"
)

func main() {
	// This map corresponds to the test cases in txsort_test.go
	testCases := map[string]struct {
		hexFile string
		name    string
	}{
		"tx100004-4.hex":   {"tx100004-4.hex", "block 100004 tx[4] - already sorted"},
		"tx101790-3.hex":   {"tx101790-3.hex", "block 101790 tx[3] - sorts inputs only, based on tree"},
		"tx150007-23.hex":  {"tx150007-23.hex", "block 150007 tx[23] - sorts inputs only, based on hash"},
		"tx108930-1.hex":   {"tx108930-1.hex", "block 108930 tx[1] - sorts inputs only, based on index"},
		"tx100082-5.hex":   {"tx100082-5.hex", "block 100082 tx[5] - sorts outputs only, based on amount"},
		"tx150043-14m.hex": {"tx150043-14m.hex", "modified block 150043 tx[14] - sorts outputs only, based on script version"},
		"tx150043-14.hex":  {"tx150043-14.hex", "block 150043 tx[14] - sorts outputs only, based on output script"},
		"tx150626-24.hex":  {"tx150626-24.hex", "block 150626 tx[24] - sorts outputs only, based on amount and output script"},
		"tx150002-7.hex":   {"tx150002-7.hex", "block 150002 tx[7] - sorts both inputs and outputs"},
	}

	testDataPath := "dcrutil/txsort/testdata"

	fmt.Println("Updated hash mappings for txsort_test.go:")
	fmt.Println()

	for filename, testCase := range testCases {
		filePath := filepath.Join(testDataPath, filename)

		// Read the hex data
		hexData, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", filePath, err)
			continue
		}

		// Decode hex
		hexStr := strings.TrimSpace(string(hexData))
		txBytes, err := hex.DecodeString(hexStr)
		if err != nil {
			fmt.Printf("Error decoding hex in %s: %v\n", filePath, err)
			continue
		}

		// Deserialize transaction with current protocol version
		var tx wire.MsgTx
		reader := bytes.NewReader(txBytes)
		err = tx.BtcDecode(reader, wire.ProtocolVersion)
		if err != nil {
			fmt.Printf("Error deserializing transaction from %s: %v\n", filePath, err)
			continue
		}

		// Get the original transaction hash
		originalHash := tx.TxHash()

		// Check if already sorted
		isSorted := txsort.IsSorted(&tx)

		// Create a copy and sort it (this is what the test does)
		sortedTx := tx.Copy()
		txsort.InPlaceSort(sortedTx)
		sortedHash := sortedTx.TxHash()

		fmt.Printf("// %s\n", testCase.name)
		fmt.Printf("%q original: \"%s\",\n", filename, originalHash.String())
		fmt.Printf("%q sorted:   \"%s\",\n", filename, sortedHash.String())
		fmt.Printf("%q isSorted: %t\n", filename, isSorted)
	}

	fmt.Println()
	fmt.Println("Use these hash values to update the expectedHashes map in txsort_test.go")
}
