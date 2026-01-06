//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/monetarium/node/dcrutil"
)

func main() {
	// Change to dcrutil directory
	os.Chdir(filepath.Join("..", "dcrutil"))

	b := dcrutil.NewBlock(&Block100000)

	// Print the block hash
	fmt.Printf("Block hash: %s\n", b.Hash())

	// Print transaction hashes
	transactions := b.Transactions()
	fmt.Printf("\nTransaction hashes:\n")
	for i, tx := range transactions {
		fmt.Printf("  [%d]: %s\n", i, tx.Hash())
	}

	fmt.Printf("\nFor updating block_test.go:\n")
	fmt.Printf("wantHashStr := \"%s\"\n", b.Hash())
	fmt.Printf("wantTxHashes := []string{\n")
	for i, tx := range transactions {
		if i == len(transactions)-1 {
			fmt.Printf("\t\"%s\",\n", tx.Hash())
		} else {
			fmt.Printf("\t\"%s\",\n", tx.Hash())
		}
	}
	fmt.Printf("}\n")
}
