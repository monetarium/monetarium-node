// +build ignore

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

func main() {
	testDataPath := "testdata"

	// Process tx432098-11.hex
	processFile(testDataPath, "tx432098-11.hex")

	// Process inline hex from DecodeRawTransaction tests
	fmt.Println("\n\n=== Processing inline transaction hexes ===")

	// First test hex
	hexStr1 := "01000000010d33d3840e9074183dc9a8d82a5031075a98135bfe182840ddaf575a" +
		"a2032fe00000000000feffffff0100e1f5050000000000000017a914f59833f104" +
		"faa3c7fd0c7dc1e3967fe77a9c152387010000000100000001010000000000000" +
		"000000000ffffffff00"
	processInlineHex("DecodeRawTransaction test 1", hexStr1)

	// Second test hex (odd length - just prepend 0)
	hexStr2 := "1000000010d33d3840e9074183dc9a8d82a5031075a98135bfe182840ddaf575aa" +
		"2032fe00000000000feffffff0100e1f5050000000000000017a914f59833f104" +
		"faa3c7fd0c7dc1e3967fe77a9c152387010000000100000001010000000000000" +
		"000000000ffffffff00"
	processInlineHex("DecodeRawTransaction test 2 (odd)", "0"+hexStr2)
}

func processInlineHex(name, hexStr string) {
	// Decode hex to bytes
	txBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Printf("Error decoding hex for %s: %v\n", name, err)
		return
	}

	fmt.Printf("\nProcessing %s\n", name)
	fmt.Printf("Original hex (%d bytes)\n", len(txBytes))

	// Deserialize with V12 format (Value first, then CoinType)
	var tx wire.MsgTx
	err = tx.BtcDecode(bytes.NewReader(txBytes), wire.DualCoinVersion)
	if err != nil {
		fmt.Printf("Error deserializing with V12: %v\n", err)
		return
	}

	// Set default CoinType for outputs if not set
	for i := range tx.TxOut {
		if tx.TxOut[i].CoinType == 0 {
			tx.TxOut[i].CoinType = cointype.CoinTypeVAR
		}
	}

	// Serialize with V13 format (CoinType first, then Value)
	var v13Buf bytes.Buffer
	tx.BtcEncode(&v13Buf, wire.ProtocolVersion)
	v13Hex := hex.EncodeToString(v13Buf.Bytes())

	// Recalculate hash with V13 format
	var tx2 wire.MsgTx
	tx2.BtcDecode(bytes.NewReader(v13Buf.Bytes()), wire.ProtocolVersion)
	v13Hash := tx2.TxHash()

	fmt.Printf("V13 format tx hash: %s\n", v13Hash)
	fmt.Printf("V13 hex (%d bytes): %s\n", len(v13Buf.Bytes()), v13Hex)
}

func processFile(testDataPath, filename string) {
	filePath := filepath.Join(testDataPath, filename)
	hexBytes, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", filename, err)
		return
	}
	hexStr := strings.TrimSpace(string(hexBytes))

	// Decode hex to bytes
	txBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Printf("Error decoding hex: %v\n", err)
		return
	}

	fmt.Printf("Processing %s\n", filename)
	fmt.Printf("Original hex (%d bytes): %s\n\n", len(txBytes), hexStr[:100]+"...")

	// Deserialize with V12 format (Value first, then CoinType)
	var tx wire.MsgTx
	err = tx.BtcDecode(bytes.NewReader(txBytes), wire.DualCoinVersion)
	if err != nil {
		fmt.Printf("Error deserializing with V12: %v\n", err)
		return
	}

	// Set default CoinType for outputs if not set
	for i := range tx.TxOut {
		if tx.TxOut[i].CoinType == 0 {
			tx.TxOut[i].CoinType = cointype.CoinTypeVAR
		}
	}

	// Calculate V12 hash
	var v12Buf bytes.Buffer
	tx.BtcEncode(&v12Buf, wire.DualCoinVersion)
	fmt.Printf("V12 format tx hash: %s\n", tx.TxHash())

	// Serialize with V13 format (CoinType first, then Value)
	var v13Buf bytes.Buffer
	tx.BtcEncode(&v13Buf, wire.ProtocolVersion)
	v13Hex := hex.EncodeToString(v13Buf.Bytes())

	// Recalculate hash with V13 format
	var tx2 wire.MsgTx
	tx2.BtcDecode(bytes.NewReader(v13Buf.Bytes()), wire.ProtocolVersion)
	v13Hash := tx2.TxHash()

	fmt.Printf("V13 format tx hash: %s\n", v13Hash)
	fmt.Printf("V13 hex (%d bytes): %s\n\n", len(v13Buf.Bytes()), v13Hex[:100]+"...")

	// Print full V13 hex for updating the test file
	fmt.Printf("\n--- V13 Hex for %s ---\n%s\n", filename, v13Hex)
	fmt.Printf("\n--- New Txid ---\n%s\n", v13Hash)

	// Compare original and V13 hex to see the difference
	fmt.Printf("\n--- Comparing output sections ---\n")
	fmt.Printf("Original: %s\n", hexStr)
	fmt.Printf("V13:      %s\n", v13Hex)

	// Show details about outputs
	fmt.Printf("\n--- Output details ---\n")
	for i, out := range tx.TxOut {
		fmt.Printf("Output %d: Value=%d, CoinType=%d, Version=%d, PkScript len=%d\n",
			i, out.Value, out.CoinType, out.Version, len(out.PkScript))
	}
}
