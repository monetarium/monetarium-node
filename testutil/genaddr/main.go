package main

import (
	"crypto/rand"
	"fmt"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/txscript/stdaddr"
)

func main() {
	// Generate random 20-byte pubkey hash
	pubKeyHash := make([]byte, 20)
	rand.Read(pubKeyHash)

	// Create simnet address
	params := chaincfg.SimNetParams()
	addr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(pubKeyHash, params)
	if err != nil {
		fmt.Println("Error creating address:", err)
		return
	}

	fmt.Println(addr.String())
}
