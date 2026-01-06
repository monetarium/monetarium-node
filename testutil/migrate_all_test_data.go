//go:build ignore

// This tool migrates all binary test data files (.bz2 and .hex) from the old
// wire format (protocol version 11, without CoinType) to the new format
// (protocol version 12, with CoinType field).
//
// Usage: go run migrate_all_test_data.go

package main

import (
	"bytes"
	"compress/bzip2"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/monetarium/node/cointype"
	"github.com/monetarium/node/wire"
)

const (
	legacyProtocolVersion = 11 // Protocol version before CoinType was added
)

// bz2FileInfo contains information about a bz2 file to migrate
type bz2FileInfo struct {
	path  string
	isGob bool // true if file contains gob-encoded map[int64][]byte
}

var bz2Files = []bz2FileInfo{
	// Database test data
	{path: "../database/testdata/blocks0to168.bz2", isGob: true},

	// Internal blockchain test data
	{path: "../internal/blockchain/testdata/blocks0to168.bz2", isGob: true},
	{path: "../internal/blockchain/testdata/reorgto179.bz2", isGob: true},

	// RPC server test data
	{path: "../internal/rpcserver/testdata/block432100.bz2", isGob: false},

	// Txscript test data
	{path: "../txscript/testdata/block432100.bz2", isGob: false},

	// Blockchain stake test data
	{path: "../blockchain/stake/testdata/blocks0to168.bz2", isGob: true},
	{path: "../blockchain/stake/testdata/testexpiry.bz2", isGob: true},
}

var hexFileDirs = []string{
	"../txscript/testdata",
	"../internal/rpcserver/testdata",
}

func main() {
	fmt.Println("=== Monetarium Test Data Migration Tool ===")
	fmt.Println("Migrating from protocol version 11 to 12 (adding CoinType field)")
	fmt.Println()

	// Migrate bz2 files
	for _, fileInfo := range bz2Files {
		if err := migrateBz2File(fileInfo); err != nil {
			fmt.Printf("ERROR migrating %s: %v\n", fileInfo.path, err)
		}
	}

	// Migrate hex files
	for _, dir := range hexFileDirs {
		if err := migrateHexFilesInDir(dir); err != nil {
			fmt.Printf("ERROR migrating hex files in %s: %v\n", dir, err)
		}
	}

	fmt.Println()
	fmt.Println("=== Migration Complete ===")
	fmt.Println("Run './run_tests.sh' to verify the migration")
}

func migrateBz2File(fileInfo bz2FileInfo) error {
	fmt.Printf("Processing %s...\n", fileInfo.path)

	// Check if file exists
	if _, err := os.Stat(fileInfo.path); os.IsNotExist(err) {
		fmt.Printf("  SKIP: File does not exist\n")
		return nil
	}

	// Create backup
	backupPath := fileInfo.path + ".legacy_v11"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := copyFile(fileInfo.path, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
		fmt.Printf("  Created backup: %s\n", backupPath)
	} else {
		fmt.Printf("  Backup already exists: %s\n", backupPath)
	}

	if fileInfo.isGob {
		return migrateGobBz2File(fileInfo.path)
	}
	return migrateSingleBlockBz2File(fileInfo.path)
}

// migrateGobBz2File migrates a bz2 file containing gob-encoded map[int64][]byte
func migrateGobBz2File(filePath string) error {
	// Open and decompress
	fi, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer fi.Close()

	bzReader := bzip2.NewReader(fi)
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(bzReader); err != nil {
		return fmt.Errorf("failed to read compressed data: %v", err)
	}

	// Decode gob
	decoder := gob.NewDecoder(buf)
	blockBytesMap := make(map[int64][]byte)
	if err := decoder.Decode(&blockBytesMap); err != nil {
		return fmt.Errorf("failed to decode gob data: %v", err)
	}

	fmt.Printf("  Found %d blocks\n", len(blockBytesMap))

	// Convert each block
	updatedMap := make(map[int64][]byte)
	for height, blockBytes := range blockBytesMap {
		var block wire.MsgBlock
		br := bytes.NewReader(blockBytes)

		// Decode using legacy protocol version
		if err := block.BtcDecode(br, legacyProtocolVersion); err != nil {
			return fmt.Errorf("failed to decode block at height %d: %v", height, err)
		}

		// Update all TxOuts with CoinType
		updateBlockCoinTypes(&block)

		// Re-encode with new protocol version
		newBuf := new(bytes.Buffer)
		if err := block.BtcEncode(newBuf, wire.ProtocolVersion); err != nil {
			return fmt.Errorf("failed to encode block at height %d: %v", height, err)
		}

		updatedMap[height] = newBuf.Bytes()
	}

	// Write updated data
	return writeGobBz2File(filePath, updatedMap)
}

// migrateSingleBlockBz2File migrates a bz2 file containing a single serialized block
func migrateSingleBlockBz2File(filePath string) error {
	// Open and decompress
	fi, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer fi.Close()

	var block wire.MsgBlock
	// Decode using legacy protocol version
	if err := block.BtcDecode(bzip2.NewReader(fi), legacyProtocolVersion); err != nil {
		return fmt.Errorf("failed to decode block: %v", err)
	}

	fmt.Printf("  Block height: %d, Txs: %d, STxs: %d\n",
		block.Header.Height, len(block.Transactions), len(block.STransactions))

	// Update all TxOuts with CoinType
	updateBlockCoinTypes(&block)

	// Write updated block
	return writeSingleBlockBz2File(filePath, &block)
}

func updateBlockCoinTypes(block *wire.MsgBlock) {
	for _, tx := range block.Transactions {
		for i := range tx.TxOut {
			tx.TxOut[i].CoinType = cointype.CoinTypeVAR
		}
	}
	for _, stx := range block.STransactions {
		for i := range stx.TxOut {
			stx.TxOut[i].CoinType = cointype.CoinTypeVAR
		}
	}
}

func writeGobBz2File(filePath string, blockBytesMap map[int64][]byte) error {
	// Create temp file for uncompressed gob data
	tempFile := filePath + ".temp"
	temp, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	// Encode gob
	encoder := gob.NewEncoder(temp)
	if err := encoder.Encode(blockBytesMap); err != nil {
		temp.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to encode gob data: %v", err)
	}
	temp.Close()

	// Compress with bzip2
	if err := compressWithBzip2(tempFile, filePath); err != nil {
		os.Remove(tempFile)
		return err
	}

	os.Remove(tempFile)
	fmt.Printf("  Successfully updated %s\n", filePath)
	return nil
}

func writeSingleBlockBz2File(filePath string, block *wire.MsgBlock) error {
	// Create temp file for raw block data
	tempFile := filePath + ".temp"
	temp, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	// Encode with new protocol version
	if err := block.BtcEncode(temp, wire.ProtocolVersion); err != nil {
		temp.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to encode block: %v", err)
	}
	temp.Close()

	// Compress with bzip2
	if err := compressWithBzip2(tempFile, filePath); err != nil {
		os.Remove(tempFile)
		return err
	}

	os.Remove(tempFile)
	fmt.Printf("  Successfully updated %s\n", filePath)
	return nil
}

func compressWithBzip2(inputFile, outputFile string) error {
	cmd := exec.Command("bzip2", "-c", inputFile)
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer output.Close()

	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bzip2 compression failed: %v", err)
	}
	return nil
}

func migrateHexFilesInDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.hex"))
	if err != nil {
		return err
	}

	for _, file := range files {
		// Skip files in legacy directories
		if strings.Contains(file, "legacy") {
			continue
		}

		if err := migrateHexFile(file); err != nil {
			fmt.Printf("ERROR migrating %s: %v\n", file, err)
		}
	}
	return nil
}

func migrateHexFile(filePath string) error {
	fmt.Printf("Processing %s...\n", filePath)

	// Read hex data
	hexData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	hexStr := strings.TrimSpace(string(hexData))
	txBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Errorf("failed to decode hex: %v", err)
	}

	// Create backup
	backupPath := filePath + ".legacy_v11"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, hexData, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
		fmt.Printf("  Created backup: %s\n", backupPath)
	}

	// Deserialize with legacy protocol version
	var tx wire.MsgTx
	reader := bytes.NewReader(txBytes)
	if err := tx.BtcDecode(reader, legacyProtocolVersion); err != nil {
		return fmt.Errorf("failed to decode transaction: %v", err)
	}

	fmt.Printf("  Tx inputs: %d, outputs: %d\n", len(tx.TxIn), len(tx.TxOut))

	// Update CoinType on all outputs
	for i := range tx.TxOut {
		tx.TxOut[i].CoinType = cointype.CoinTypeVAR
	}

	// Serialize with new protocol version
	newTxBytes, err := tx.Bytes()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %v", err)
	}

	// Write updated hex
	newHexStr := hex.EncodeToString(newTxBytes)
	if err := os.WriteFile(filePath, []byte(newHexStr), 0644); err != nil {
		return fmt.Errorf("failed to write updated file: %v", err)
	}

	fmt.Printf("  Successfully updated %s\n", filePath)
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
