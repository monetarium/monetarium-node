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
	testCases := []string{
		"tx100004-4.hex",
		"tx101790-3.hex",
		"tx150007-23.hex",
		"tx108930-1.hex",
		"tx100082-5.hex",
		"tx150043-14m.hex",
		"tx150043-14.hex",
		"tx150626-24.hex",
		"tx150002-7.hex",
	}

	testDataPath := "dcrutil/txsort/testdata"

	fmt.Println("Transaction sorting analysis:")
	fmt.Println()

	for _, filename := range testCases {
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

		// Check if already sorted
		originalHash := tx.TxHash()
		isSorted := txsort.IsSorted(&tx)

		// Make a copy and sort it
		sortedTx := tx.Copy()
		txsort.Sort(sortedTx)
		sortedHash := sortedTx.TxHash()

		sameAfterSort := originalHash.IsEqual(&sortedHash)

		fmt.Printf("%s:\n", filename)
		fmt.Printf("  Original hash: %s\n", originalHash.String())
		fmt.Printf("  Sorted hash:   %s\n", sortedHash.String())
		fmt.Printf("  IsSorted:      %t\n", isSorted)
		fmt.Printf("  Same after sort: %t\n", sameAfterSort)
		fmt.Printf("  Recommendation: isSorted: %t, both hashes: %s\n", sameAfterSort, originalHash.String())
		fmt.Println()
	}
}
