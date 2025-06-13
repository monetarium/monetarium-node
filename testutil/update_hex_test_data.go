//go:build ignore

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/decred/dcrd/wire"
)

func main() {
	// Find all hex test data files in rpcserver legacy testdata
	testDataPath := "../internal/rpcserver/testdata"
	legacyDataPath := filepath.Join(testDataPath, "legacy_v11")
	
	files, err := filepath.Glob(filepath.Join(legacyDataPath, "*.hex"))
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
		
		// Deserialize with old protocol version (11) for legacy test data
		var tx wire.MsgTx
		reader := bytes.NewReader(txBytes)
		err = tx.BtcDecode(reader, 11)
		if err != nil {
			fmt.Printf("Error deserializing transaction from %s: %v\n", file, err)
			continue
		}
		
		// Ensure all TxOut entries have CoinType set to VAR
		for i := range tx.TxOut {
			tx.TxOut[i].CoinType = wire.CoinTypeVAR
		}
		
		// Serialize with current protocol version
		newTxBytes, err := tx.Bytes()
		if err != nil {
			fmt.Printf("Error serializing transaction for %s: %v\n", file, err)
			continue
		}
		
		// Encode as hex
		newHexStr := hex.EncodeToString(newTxBytes)
		
		// Write updated file to main testdata directory
		outputFile := filepath.Join(testDataPath, filepath.Base(file))
		err = os.WriteFile(outputFile, []byte(newHexStr), 0644)
		if err != nil {
			fmt.Printf("Error writing updated %s: %v\n", file, err)
			continue
		}
		
		fmt.Printf("Updated %s -> %s successfully\n", file, outputFile)
	}
	
	fmt.Println("Done updating hex test data files")
}