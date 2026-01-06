//go:build ignore

package main

import (
	"bytes"
	"compress/bzip2"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

func main() {
	// Path to the stake test data directory
	stakeTestDataPath := "../blockchain/stake/testdata"

	// Test data files to regenerate
	testFiles := []string{
		"blocks0to168.bz2",
		"testexpiry.bz2",
	}

	for _, filename := range testFiles {
		fmt.Printf("Processing %s...\n", filename)
		if err := regenerateTestFile(stakeTestDataPath, filename); err != nil {
			fmt.Printf("Error processing %s: %v\n", filename, err)
			continue
		}
		fmt.Printf("Successfully regenerated %s\n", filename)
	}
}

func regenerateTestFile(testDataPath, filename string) error {
	originalFile := filepath.Join(testDataPath, filename)
	backupFile := filepath.Join(testDataPath, filename+".backup")

	// Create backup of original file
	if err := copyFile(originalFile, backupFile); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Open and read the original file
	fi, err := os.Open(originalFile)
	if err != nil {
		return fmt.Errorf("failed to open original file: %v", err)
	}
	defer fi.Close()

	// The stake test files contain multiple blocks serialized together
	// We need to process them one by one
	bzReader := bzip2.NewReader(fi)

	// Read all blocks from the compressed data
	blocks, err := readAllBlocksFromCompressed(bzReader)
	if err != nil {
		return fmt.Errorf("failed to read blocks: %v", err)
	}

	fmt.Printf("  Found %d blocks in %s\n", len(blocks), filename)

	// Update all blocks to include CoinType fields
	for height, block := range blocks {
		updateBlockForDualCoin(block)
		fmt.Printf("  Updated block at height %d with dual-coin CoinType fields\n", height)
	}

	// Write the updated blocks back to compressed format
	if err := writeAllBlocksToCompressed(originalFile, blocks); err != nil {
		return fmt.Errorf("failed to write updated blocks: %v", err)
	}

	return nil
}

func readAllBlocksFromCompressed(reader io.Reader) (map[int64]*wire.MsgBlock, error) {
	// Read all data into buffer
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %v", err)
	}

	// Decode the gob-encoded map
	decoder := gob.NewDecoder(buf)
	testBlockchainBytes := make(map[int64][]byte)

	if err := decoder.Decode(&testBlockchainBytes); err != nil {
		return nil, fmt.Errorf("failed to decode gob data: %v", err)
	}

	// Convert byte map to block map
	blocks := make(map[int64]*wire.MsgBlock)

	for height, blockBytes := range testBlockchainBytes {
		var block wire.MsgBlock
		br := bytes.NewReader(blockBytes)

		// Decode using legacy protocol version (pre-dual-coin)
		if err := block.BtcDecode(br, 11); err != nil {
			return nil, fmt.Errorf("failed to decode block at height %d: %v", height, err)
		}

		blocks[height] = &block
	}

	return blocks, nil
}

func updateBlockForDualCoin(block *wire.MsgBlock) {
	// Update all regular transactions
	for _, tx := range block.Transactions {
		for i := range tx.TxOut {
			tx.TxOut[i].CoinType = cointype.CoinTypeVAR
		}
	}

	// Update all stake transactions
	for _, stx := range block.STransactions {
		for i := range stx.TxOut {
			stx.TxOut[i].CoinType = cointype.CoinTypeVAR
		}
	}
}

func writeAllBlocksToCompressed(outputPath string, blocks map[int64]*wire.MsgBlock) error {
	// Convert blocks back to byte map format
	updatedBlockBytes := make(map[int64][]byte)

	for height, block := range blocks {
		// Serialize the block with current protocol version (dual-coin aware)
		buf := new(bytes.Buffer)
		if err := block.BtcEncode(buf, wire.ProtocolVersion); err != nil {
			return fmt.Errorf("failed to encode block at height %d: %v", height, err)
		}
		updatedBlockBytes[height] = buf.Bytes()
	}

	// Create temporary file for uncompressed data
	tempFile := outputPath + ".temp"
	temp, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer func() {
		temp.Close()
		os.Remove(tempFile)
	}()

	// Encode the map using gob
	encoder := gob.NewEncoder(temp)
	if err := encoder.Encode(updatedBlockBytes); err != nil {
		return fmt.Errorf("failed to encode gob data: %v", err)
	}

	temp.Close()

	// Compress the temp file using external bzip2 command
	// This is necessary because Go's standard library doesn't have a bzip2 writer
	return compressWithBzip2(tempFile, outputPath)
}

func compressWithBzip2(inputFile, outputFile string) error {
	// Read the uncompressed data
	input, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read temp file: %v", err)
	}

	// For now, we'll create an uncompressed version and let the user compress it manually
	// This is because Go's standard library doesn't include a bzip2 writer
	uncompressedFile := outputFile + ".uncompressed"
	if err := os.WriteFile(uncompressedFile, input, 0644); err != nil {
		return fmt.Errorf("failed to write uncompressed file: %v", err)
	}

	fmt.Printf("  Created uncompressed version: %s\n", uncompressedFile)
	fmt.Printf("  To complete the update, run: bzip2 -c %s > %s\n", uncompressedFile, outputFile)

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
