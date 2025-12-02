// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"bytes"

	"github.com/decred/dcrd/cointype"
	"github.com/decred/dcrd/internal/blockchain"
	"github.com/decred/dcrd/wire"
)

// findAugmentableUTXO searches for an existing UTXO that can be augmented
// with fees for the given recipient pkScript and coin type.
//
// The function prioritizes smaller UTXOs to maximize consolidation benefit.
// It returns nil outpoint and entry if no suitable UTXO is found.
//
// This is used to prevent dust UTXO accumulation by reusing existing outputs
// instead of creating new ones from null inputs.
func findAugmentableUTXO(
	utxoView *blockchain.UtxoViewpoint,
	recipientPkScript []byte,
	coinType cointype.CoinType,
) (*wire.OutPoint, *blockchain.UtxoEntry) {

	if utxoView == nil || recipientPkScript == nil {
		return nil, nil
	}

	// Iterate through UTXO set filtered by coin type
	entries := utxoView.LookupEntriesByCoinType(coinType)

	var bestOutpoint *wire.OutPoint
	var bestEntry *blockchain.UtxoEntry
	var smallestValue int64 = -1

	for outpoint, entry := range entries {
		if entry == nil || entry.IsSpent() {
			continue
		}

		// Match by pkScript (recipient address)
		if !bytes.Equal(entry.PkScript(), recipientPkScript) {
			continue
		}

		// Prefer smallest UTXO to maximize consolidation
		if smallestValue < 0 || entry.Amount() < smallestValue {
			// Make a copy of the outpoint to avoid pointer issues
			outpointCopy := outpoint
			bestOutpoint = &outpointCopy
			bestEntry = entry
			smallestValue = entry.Amount()
		}
	}

	return bestOutpoint, bestEntry
}
