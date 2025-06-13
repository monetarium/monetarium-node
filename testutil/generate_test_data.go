//go:build ignore

package main

import (
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/decred/dcrd/wire"
)

func main() {
	// Load the original block432100.bz2 file
	testDataPath := "./internal/rpcserver/testdata"
	blockDataFile := filepath.Join(testDataPath, "block432100.bz2")
	
	fi, err := os.Open(blockDataFile)
	if err != nil {
		panic(err)
	}
	defer fi.Close()
	
	var block wire.MsgBlock
	// Use protocol version 11 to read the legacy test data
	err = block.BtcDecode(bzip2.NewReader(fi), 11)
	if err != nil {
		panic(err)
	}
	
	// Ensure all TxOut entries have CoinType set to VAR for consistency with current protocol
	for _, tx := range block.Transactions {
		for i := range tx.TxOut {
			tx.TxOut[i].CoinType = wire.CoinTypeVAR
		}
	}
	for _, stx := range block.STransactions {
		for i := range stx.TxOut {
			stx.TxOut[i].CoinType = wire.CoinTypeVAR
		}
	}
	
	// Create a backup of the original file
	backupFile := filepath.Join(testDataPath, "block432100.bz2.backup")
	originalFile, err := os.Open(blockDataFile)
	if err != nil {
		panic(err)
	}
	defer originalFile.Close()
	
	backup, err := os.Create(backupFile)
	if err != nil {
		panic(err)
	}
	defer backup.Close()
	
	// Copy original to backup
	_, err = backup.ReadFrom(originalFile)
	if err != nil {
		panic(err)
	}
	
	// Now write the updated block with CoinType fields using current protocol version
	outputFile := filepath.Join(testDataPath, "block432100_new.bz2")
	fo, err := os.Create(outputFile)
	if err != nil {
		panic(err)
	}
	defer fo.Close()
	
	// We need to create a bzip2 writer, but Go's standard library only has a reader
	// Let's use gzip instead for now, or write raw bytes and compress externally
	
	// Write raw serialized data first
	tempFile := filepath.Join(testDataPath, "block432100_temp.dat")
	temp, err := os.Create(tempFile)
	if err != nil {
		panic(err)
	}
	defer temp.Close()
	
	// Serialize with current protocol version to include CoinType field
	err = block.BtcEncode(temp, wire.ProtocolVersion)
	if err != nil {
		panic(err)
	}
	
	temp.Close()
	
	// Now create a gzip version for testing
	gzipFile := filepath.Join(testDataPath, "block432100_new.gz")
	gzOut, err := os.Create(gzipFile)
	if err != nil {
		panic(err)
	}
	defer gzOut.Close()
	
	gzWriter := gzip.NewWriter(gzOut)
	defer gzWriter.Close()
	
	// Re-open temp file and copy to gzip
	tempRead, err := os.Open(tempFile)
	if err != nil {
		panic(err)
	}
	defer tempRead.Close()
	
	_, err = io.Copy(gzWriter, tempRead)
	if err != nil {
		panic(err)
	}
	
	// Clean up temp file
	os.Remove(tempFile)
	
	println("Successfully generated test data files:")
	println("- Backup:", backupFile)
	println("- New compressed data:", gzipFile)
	println("- Raw temp data was cleaned up")
	println()
	println("To complete the update:")
	println("1. Compress the raw data with bzip2 externally if needed")
	println("2. Replace the original block432100.bz2 with the new version")
	println("3. Update test code to use current protocol version")
}