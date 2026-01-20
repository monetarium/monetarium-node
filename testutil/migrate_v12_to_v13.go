//go:build ignore

package main

import (
	"bytes"
	"compress/bzip2"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/wire"
)

func main() {
	// Path to the stake test data directory
	stakeTestDataPath := "../blockchain/stake/testdata"

	// Test data files to migrate from v12 to v13
	testFiles := []string{
		"blocks0to168.bz2",
		"testexpiry.bz2",
	}

	for _, filename := range testFiles {
		fmt.Printf("Processing %s...\n", filename)
		if err := migrateTestFile(stakeTestDataPath, filename); err != nil {
			fmt.Printf("Error processing %s: %v\n", filename, err)
			continue
		}
		fmt.Printf("Successfully migrated %s to v13 format\n", filename)
	}
}

func migrateTestFile(testDataPath, filename string) error {
	originalFile := filepath.Join(testDataPath, filename)
	backupFile := filepath.Join(testDataPath, filename+".v12_backup")

	// Try to restore from backup if it exists (for re-running migration)
	if _, err := os.Stat(backupFile); err == nil {
		fmt.Printf("  Restoring from backup before migration...\n")
		if err := copyFile(backupFile, originalFile); err != nil {
			return fmt.Errorf("failed to restore from backup: %v", err)
		}
	}

	// Create backup of original file
	if err := copyFile(originalFile, backupFile); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}
	fmt.Printf("  Created backup: %s\n", backupFile)

	// Open and read the original file
	fi, err := os.Open(originalFile)
	if err != nil {
		return fmt.Errorf("failed to open original file: %v", err)
	}
	defer fi.Close()

	// Read compressed data
	bzReader := bzip2.NewReader(fi)

	// Read all data into buffer
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(bzReader); err != nil {
		return fmt.Errorf("failed to read compressed data: %v", err)
	}

	// Decode the gob-encoded map
	decoder := gob.NewDecoder(buf)
	testBlockchainBytes := make(map[int64][]byte)

	if err := decoder.Decode(&testBlockchainBytes); err != nil {
		return fmt.Errorf("failed to decode gob data: %v", err)
	}

	fmt.Printf("  Found %d blocks in %s\n", len(testBlockchainBytes), filename)

	// First pass: Decode all blocks and build hash mapping
	blocks := make(map[int64]*wire.MsgBlock)
	hashMap := make(map[chainhash.Hash]chainhash.Hash) // v12 hash -> v13 hash

	for height, blockBytes := range testBlockchainBytes {
		var block wire.MsgBlock
		br := bytes.NewReader(blockBytes)

		// Decode using DualCoinVersion (v12)
		if err := block.BtcDecode(br, wire.DualCoinVersion); err != nil {
			return fmt.Errorf("failed to decode block at height %d: %v", height, err)
		}

		blocks[height] = &block

		// Build hash mapping for all transactions
		for _, tx := range block.Transactions {
			v12Hash := computeTxHashV12(tx)
			v13Hash := computeTxHashV13(tx)
			if v12Hash != v13Hash {
				hashMap[v12Hash] = v13Hash
			}
		}
		for _, tx := range block.STransactions {
			v12Hash := computeTxHashV12(tx)
			v13Hash := computeTxHashV13(tx)
			if v12Hash != v13Hash {
				hashMap[v12Hash] = v13Hash
			}
		}
	}

	fmt.Printf("  Built hash mapping with %d entries\n", len(hashMap))

	// Second pass: Update all PreviousOutPoint.Hash references
	updatedRefs := 0
	for _, block := range blocks {
		for _, tx := range block.Transactions {
			for i := range tx.TxIn {
				if newHash, ok := hashMap[tx.TxIn[i].PreviousOutPoint.Hash]; ok {
					tx.TxIn[i].PreviousOutPoint.Hash = newHash
					updatedRefs++
				}
			}
		}
		for _, tx := range block.STransactions {
			for i := range tx.TxIn {
				if newHash, ok := hashMap[tx.TxIn[i].PreviousOutPoint.Hash]; ok {
					tx.TxIn[i].PreviousOutPoint.Hash = newHash
					updatedRefs++
				}
			}
		}
	}

	fmt.Printf("  Updated %d transaction input references\n", updatedRefs)

	// Third pass: Also update block header PrevBlock references
	// (block hashes also change because they include merkle root which includes tx hashes)
	// For now we'll just re-encode blocks - the test uses blocks by height, not by hash

	// Fourth pass: Encode all blocks with v13 format
	updatedBlockBytes := make(map[int64][]byte)
	for height, block := range blocks {
		newBuf := new(bytes.Buffer)
		if err := block.BtcEncode(newBuf, wire.SKABigIntVersion); err != nil {
			return fmt.Errorf("failed to encode block at height %d: %v", height, err)
		}
		updatedBlockBytes[height] = newBuf.Bytes()
	}

	fmt.Printf("  Converted all blocks to v13 format\n")

	// Write the updated blocks back to compressed format
	if err := writeAllBlocksToCompressed(originalFile, updatedBlockBytes); err != nil {
		return fmt.Errorf("failed to write updated blocks: %v", err)
	}

	return nil
}

// computeTxHashWithVersion computes the transaction hash using a specific protocol version
// TxHash uses TxSerializeNoWitness mode, so we need to do the same
func computeTxHashWithVersion(tx *wire.MsgTx, pver uint32) chainhash.Hash {
	// Make a shallow copy to avoid modifying the original
	txCopy := *tx
	// Set serialization type to NoWitness (this is what TxHash uses)
	txCopy.SerType = wire.TxSerializeNoWitness

	buf := new(bytes.Buffer)
	txCopy.BtcEncode(buf, pver)
	return chainhash.HashH(buf.Bytes())
}

// computeTxHashV12 computes the transaction hash using v12 (DualCoinVersion) serialization
func computeTxHashV12(tx *wire.MsgTx) chainhash.Hash {
	return computeTxHashWithVersion(tx, wire.DualCoinVersion)
}

// computeTxHashV13 computes the transaction hash using v13 (SKABigIntVersion) serialization
func computeTxHashV13(tx *wire.MsgTx) chainhash.Hash {
	return computeTxHashWithVersion(tx, wire.SKABigIntVersion)
}

func writeAllBlocksToCompressed(outputPath string, blockBytes map[int64][]byte) error {
	// Create temporary file for uncompressed gob data
	tempFile := outputPath + ".temp"
	temp, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	// Encode the map using gob
	encoder := gob.NewEncoder(temp)
	if err := encoder.Encode(blockBytes); err != nil {
		temp.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to encode gob data: %v", err)
	}
	temp.Close()

	// Compress using external bzip2 command
	if err := compressWithBzip2(tempFile, outputPath); err != nil {
		os.Remove(tempFile)
		return err
	}

	// Cleanup temp file
	os.Remove(tempFile)

	return nil
}

func compressWithBzip2(inputFile, outputFile string) error {
	// Use external bzip2 command since Go stdlib doesn't have a bzip2 writer
	cmd := exec.Command("bzip2", "-c", inputFile)
	outFile, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bzip2 compression failed: %v", err)
	}

	fmt.Printf("  Compressed and wrote: %s\n", outputFile)
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
