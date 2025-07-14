//go:build ignore

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/decred/dcrd/wire"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run debug_tx.go <hex_file>")
		os.Exit(1)
	}

	// Read the hex file
	hexData, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	// Remove any whitespace
	hexStr := string(bytes.TrimSpace(hexData))

	// Decode hex to bytes
	txBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Transaction hex length: %d bytes\n", len(txBytes))
	fmt.Printf("First 32 bytes: %x\n", txBytes[:min(32, len(txBytes))])

	// Try to decode with different protocol versions
	for pver := uint32(10); pver <= 12; pver++ {
		fmt.Printf("\n--- Trying protocol version %d ---\n", pver)

		var tx wire.MsgTx
		reader := bytes.NewReader(txBytes)
		err := tx.BtcDecode(reader, pver)

		if err != nil {
			fmt.Printf("Error with protocol version %d: %v\n", pver, err)

			// Check if we can read at least the prefix
			reader = bytes.NewReader(txBytes)
			fmt.Printf("Trying to decode prefix only...\n")

			// Try to manually read the transaction prefix to see how many inputs there are
			version := make([]byte, 4)
			_, err := reader.Read(version)
			if err != nil {
				fmt.Printf("Failed to read version: %v\n", err)
				continue
			}
			fmt.Printf("Version bytes: %x\n", version)

			// Read input count
			inputCount, err := wire.ReadVarInt(reader, pver)
			if err != nil {
				fmt.Printf("Failed to read input count: %v\n", err)
				continue
			}
			fmt.Printf("Input count: %d\n", inputCount)

			// Skip inputs to get to outputs
			for i := uint64(0); i < inputCount; i++ {
				// Skip previous outpoint (32 + 4 + 1 bytes)
				reader.Seek(37, io.SeekCurrent)
				// Skip signature script
				scriptLen, err := wire.ReadVarInt(reader, pver)
				if err != nil {
					fmt.Printf("Failed to read script length for input %d: %v\n", i, err)
					break
				}
				reader.Seek(int64(scriptLen), io.SeekCurrent)
				// Skip sequence (4 bytes)
				reader.Seek(4, io.SeekCurrent)
			}

			// Read output count
			outputCount, err := wire.ReadVarInt(reader, pver)
			if err != nil {
				fmt.Printf("Failed to read output count: %v\n", err)
				continue
			}
			fmt.Printf("Output count: %d\n", outputCount)

		} else {
			fmt.Printf("Success! Transaction has %d inputs, %d outputs\n",
				len(tx.TxIn), len(tx.TxOut))

			// Print coin types if available
			for i, out := range tx.TxOut {
				fmt.Printf("Output %d: Value=%d, CoinType=%d\n",
					i, out.Value, out.CoinType)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
