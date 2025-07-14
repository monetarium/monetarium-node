//go:build ignore

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/decred/dcrd/wire"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: go run get_tx_hex.go <hex_file>")
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

	// Decode transaction with current protocol version
	var tx wire.MsgTx
	reader := bytes.NewReader(txBytes)
	err = tx.BtcDecode(reader, wire.ProtocolVersion)
	if err != nil {
		panic(err)
	}

	// Re-encode transaction with current protocol version
	var buf bytes.Buffer
	err = tx.BtcEncode(&buf, wire.ProtocolVersion)
	if err != nil {
		panic(err)
	}

	// Output the new hex string
	newHex := hex.EncodeToString(buf.Bytes())
	fmt.Println("Length:", len(newHex))
	fmt.Println("Hex:", newHex)
}
