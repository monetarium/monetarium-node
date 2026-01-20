//go:build ignore

// This tool migrates JSON test data files that contain serialized transactions
// from the old wire format (protocol version 11) to the new format (protocol
// version 12 with CoinType field).
//
// Files processed:
// - txscript/testdata/tx_valid.json
// - txscript/testdata/tx_invalid.json
// - txscript/testdata/sighash.json
//
// Usage: go run migrate_json_test_data.go

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/txscript"
	"github.com/monetarium/monetarium-node/wire"
)

const (
	legacyProtocolVersion = 11
)

func main() {
	fmt.Println("=== JSON Test Data Migration Tool ===")
	fmt.Println()

	// Migrate tx_valid.json and tx_invalid.json
	txFiles := []string{
		"./txscript/testdata/tx_valid.json",
		"./txscript/testdata/tx_invalid.json",
	}

	for _, file := range txFiles {
		if err := migrateTxTestFile(file); err != nil {
			fmt.Printf("ERROR migrating %s: %v\n", file, err)
		}
	}

	// Migrate sighash.json
	if err := migrateSigHashFile("./txscript/testdata/sighash.json"); err != nil {
		fmt.Printf("ERROR migrating sighash.json: %v\n", err)
	}

	fmt.Println()
	fmt.Println("=== JSON Migration Complete ===")
}

// migrateTxTestFile migrates tx_valid.json or tx_invalid.json
// Format: [[[prevOutHash, prevOutIndex, prevOutScript]...], serializedTxHex, verifyFlags]
func migrateTxTestFile(filePath string) error {
	fmt.Printf("Processing %s...\n", filePath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Create backup
	backupPath := filePath + ".legacy_v11"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
		fmt.Printf("  Created backup: %s\n", backupPath)
	}

	// Parse JSON
	var tests [][]interface{}
	if err := json.Unmarshal(data, &tests); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	updatedCount := 0
	for i, test := range tests {
		// Skip comments (single element arrays)
		if len(test) == 1 {
			continue
		}

		// Format: [inputs, serializedTxHex, flags]
		if len(test) < 3 {
			continue
		}

		// Get the serialized transaction (second element)
		txHex, ok := test[1].(string)
		if !ok {
			continue
		}

		// Decode hex
		txBytes, err := hex.DecodeString(txHex)
		if err != nil {
			fmt.Printf("  Warning: test %d has invalid hex\n", i)
			continue
		}

		// Deserialize with legacy protocol version
		var tx wire.MsgTx
		reader := bytes.NewReader(txBytes)
		if err := tx.BtcDecode(reader, legacyProtocolVersion); err != nil {
			fmt.Printf("  Warning: test %d failed to decode: %v\n", i, err)
			continue
		}

		// Update CoinType on all outputs
		for j := range tx.TxOut {
			tx.TxOut[j].CoinType = cointype.CoinTypeVAR
		}

		// Serialize with new protocol version
		newTxBytes, err := tx.Bytes()
		if err != nil {
			fmt.Printf("  Warning: test %d failed to serialize: %v\n", i, err)
			continue
		}

		// Update the test entry
		tests[i][1] = hex.EncodeToString(newTxBytes)
		updatedCount++
	}

	// Write updated JSON
	output, err := json.MarshalIndent(tests, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(filePath, output, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	fmt.Printf("  Updated %d test transactions in %s\n", updatedCount, filePath)
	return nil
}

// migrateSigHashFile migrates sighash.json
// Format: [txHex, scriptHex, inputIdx, hashType, expectedHashHex, expectedError, comment]
func migrateSigHashFile(filePath string) error {
	fmt.Printf("Processing %s...\n", filePath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Create backup
	backupPath := filePath + ".legacy_v11"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
		fmt.Printf("  Created backup: %s\n", backupPath)
	}

	// Parse JSON
	var tests [][]interface{}
	if err := json.Unmarshal(data, &tests); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	updatedCount := 0
	for i, test := range tests {
		// Skip comments (single element arrays)
		if len(test) == 1 {
			continue
		}

		// Format: [txHex, scriptHex, inputIdx, hashType, expectedHash, expectedError, comment]
		if len(test) < 6 {
			continue
		}

		// Get the transaction hex (first element)
		txHex, ok := test[0].(string)
		if !ok {
			continue
		}

		// Decode hex
		txBytes, err := hex.DecodeString(txHex)
		if err != nil {
			fmt.Printf("  Warning: test %d has invalid tx hex\n", i)
			continue
		}

		// Deserialize with legacy protocol version
		var tx wire.MsgTx
		reader := bytes.NewReader(txBytes)
		if err := tx.BtcDecode(reader, legacyProtocolVersion); err != nil {
			fmt.Printf("  Warning: test %d failed to decode tx: %v\n", i, err)
			continue
		}

		// Update CoinType on all outputs
		for j := range tx.TxOut {
			tx.TxOut[j].CoinType = cointype.CoinTypeVAR
		}

		// Serialize with new protocol version
		newTxBytes, err := tx.Bytes()
		if err != nil {
			fmt.Printf("  Warning: test %d failed to serialize tx: %v\n", i, err)
			continue
		}

		// Update the transaction hex
		tests[i][0] = hex.EncodeToString(newTxBytes)

		// Get script hex
		scriptHex, ok := test[1].(string)
		if !ok {
			continue
		}
		script, err := hex.DecodeString(scriptHex)
		if err != nil {
			continue
		}

		// Get input index
		inputIdxFloat, ok := test[2].(float64)
		if !ok {
			continue
		}
		inputIdx := int(inputIdxFloat)

		// Get hash type
		hashTypeFloat, ok := test[3].(float64)
		if !ok {
			continue
		}
		hashType := txscript.SigHashType(int32(hashTypeFloat))

		// Get expected error
		expectedErr, ok := test[5].(string)
		if !ok {
			continue
		}

		// Only recalculate hash if the test expects success (OK)
		if expectedErr == "OK" {
			// Recalculate signature hash with new format
			newHash, err := txscript.CalcSignatureHash(script, hashType, &tx, inputIdx, nil)
			if err != nil {
				fmt.Printf("  Warning: test %d failed to calc sig hash: %v\n", i, err)
				continue
			}

			// Update expected hash
			tests[i][4] = hex.EncodeToString(newHash)
		}

		updatedCount++
	}

	// Write updated JSON
	output, err := json.MarshalIndent(tests, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(filePath, output, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	fmt.Printf("  Updated %d tests in %s\n", updatedCount, filePath)
	return nil
}
