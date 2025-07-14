//go:build ignore

package main

import (
	"compress/bzip2"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/decred/dcrd/wire"
)

func main() {
	// Load the legacy block432100.bz2 file
	testDataPath := "../internal/rpcserver/testdata"
	legacyDataPath := filepath.Join(testDataPath, "legacy_v11")
	blockDataFile := filepath.Join(legacyDataPath, "block432100.bz2")

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

	// Write the updated block to a raw file first
	rawFile := filepath.Join(testDataPath, "block432100_raw.dat")
	raw, err := os.Create(rawFile)
	if err != nil {
		panic(err)
	}

	// Serialize with current protocol version to include CoinType field
	err = block.BtcEncode(raw, wire.ProtocolVersion)
	if err != nil {
		panic(err)
	}
	raw.Close()

	// Compress with bzip2 using external command
	outputFile := filepath.Join(testDataPath, "block432100.bz2")
	cmd := exec.Command("bzip2", "-c", rawFile)

	output, err := os.Create(outputFile)
	if err != nil {
		panic(err)
	}
	defer output.Close()

	cmd.Stdout = output
	err = cmd.Run()
	if err != nil {
		panic(err)
	}

	// Clean up raw file
	os.Remove(rawFile)

	println("Successfully updated", outputFile)
	println("The file now includes CoinType fields and uses protocol version", wire.ProtocolVersion)
	println("Legacy version is preserved in:", blockDataFile)
}
