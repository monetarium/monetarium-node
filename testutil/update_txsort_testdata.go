//go:build ignore

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monetarium/node/cointype"
	"github.com/monetarium/node/wire"
)

func main() {
	// Find all hex test data files in txsort testdata
	testDataPath := "dcrutil/txsort/testdata"

	files, err := filepath.Glob(filepath.Join(testDataPath, "*.hex"))
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		fmt.Printf("Processing %s...\n", file)

		// Read the hex data
		hexData, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", file, err)
			continue
		}

		// Decode hex
		hexStr := strings.TrimSpace(string(hexData))
		txBytes, err := hex.DecodeString(hexStr)
		if err != nil {
			fmt.Printf("Error decoding hex in %s: %v\n", file, err)
			continue
		}

		// Deserialize with legacy protocol version for old transaction data
		var tx wire.MsgTx
		reader := bytes.NewReader(txBytes)

		// Try with protocol version 11 first (legacy), then fallback to auto-detection
		err = tx.BtcDecode(reader, 11)
		if err != nil {
			// Try with current protocol version
			reader = bytes.NewReader(txBytes)
			err = tx.BtcDecode(reader, wire.ProtocolVersion)
			if err != nil {
				fmt.Printf("Error deserializing transaction from %s: %v\n", file, err)
				continue
			}
		} else {
			// Legacy transaction data - need to add CoinType field for compatibility
			for i := range tx.TxOut {
				tx.TxOut[i].CoinType = cointype.CoinTypeVAR
			}
		}

		// Serialize with current protocol version
		newTxBytes, err := tx.Bytes()
		if err != nil {
			fmt.Printf("Error serializing transaction for %s: %v\n", file, err)
			continue
		}

		// Encode as hex
		newHexStr := hex.EncodeToString(newTxBytes)

		// Write updated file
		err = os.WriteFile(file, []byte(newHexStr), 0644)
		if err != nil {
			fmt.Printf("Error writing updated %s: %v\n", file, err)
			continue
		}

		fmt.Printf("Updated %s successfully (old: %d bytes, new: %d bytes)\n",
			file, len(txBytes), len(newTxBytes))
		fmt.Printf("Old hash: %x\n", tx.TxHash())
	}

	fmt.Println("Done updating txsort test data files")
}
