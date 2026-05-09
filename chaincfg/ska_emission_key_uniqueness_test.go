// Copyright (c) 2026 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package chaincfg

import (
	"encoding/hex"
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
)

// TestSKAEmissionKeysUnique enforces the invariant that no two SKACoin
// entries on the same network share an EmissionKey.
//
// Sharing an EmissionKey across coin types is a CRITICAL-class deployment
// hazard: the holder of the shared key controls emission for every coin
// type that lists it. In particular, an "already-emitted" coin's key cannot
// be safely reused for a future coin — the historical key is observable on
// chain and any party who custodied it can mint the future coin the moment
// it activates.
//
// This test is the CI-level forcing function for the 2026-05-02 review's
// CRITICAL finding (mainnet SKA-2 placeholder key identical to SKA-1's).
// When this test fails, the fix is a real key ceremony for the duplicating
// coin type — never a code change to silence the test.
func TestSKAEmissionKeysUnique(t *testing.T) {
	networks := []struct {
		name   string
		params *Params
	}{
		{"mainnet", MainNetParams()},
		{"testnet", TestNet3Params()},
		{"simnet", SimNetParams()},
		{"regnet", RegNetParams()},
	}

	for _, n := range networks {
		t.Run(n.name, func(t *testing.T) {
			seen := make(map[string][]cointype.CoinType)
			for ct, cfg := range n.params.SKACoins {
				if cfg == nil || cfg.EmissionKey == nil {
					continue
				}
				keyHex := hex.EncodeToString(cfg.EmissionKey.SerializeCompressed())
				seen[keyHex] = append(seen[keyHex], ct)
			}
			for keyHex, cts := range seen {
				if len(cts) <= 1 {
					continue
				}
				t.Errorf("EmissionKey %s is shared by SKA coin types %v on %s — "+
					"each SKA coin type MUST have a unique emission key produced "+
					"by an independent key ceremony. Replace the duplicate "+
					"keys in chaincfg before any release that activates them.",
					keyHex, cts, n.name)
			}
		})
	}
}
