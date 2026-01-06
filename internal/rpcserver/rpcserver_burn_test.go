// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcserver

import (
	"context"
	"testing"

	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/rpc/jsonrpc/types"
)

// Helper function to create testRPCChain with burn amounts
func newTestRPCChainWithBurns(burnedAmounts map[cointype.CoinType]int64) *testRPCChain {
	return &testRPCChain{
		skaBurnedAmounts: burnedAmounts,
	}
}

// TestHandleGetBurnedCoins tests the handleGetBurnedCoins RPC handler.
func TestHandleGetBurnedCoins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cmd           *types.GetBurnedCoinsCmd
		burnedAmounts map[cointype.CoinType]int64
		wantErr       bool
		wantStatsLen  int
		validate      func(t *testing.T, result interface{})
	}{
		{
			name: "query specific coin type with burns",
			cmd: &types.GetBurnedCoinsCmd{
				CoinType: uint8Ptr(1),
			},
			burnedAmounts: map[cointype.CoinType]int64{
				1: 100000000000, // 1000 coins
			},
			wantErr:      false,
			wantStatsLen: 1,
			validate: func(t *testing.T, result interface{}) {
				r := result.(types.GetBurnedCoinsResult)
				if len(r.Stats) != 1 {
					t.Errorf("expected 1 stat, got %d", len(r.Stats))
					return
				}
				if r.Stats[0].CoinType != 1 {
					t.Errorf("expected coin type 1, got %d", r.Stats[0].CoinType)
				}
				if r.Stats[0].TotalBurned != 1000.0 {
					t.Errorf("expected total burned 1000.0, got %f", r.Stats[0].TotalBurned)
				}
				if r.Stats[0].Name != "SKA-1" {
					t.Errorf("expected name SKA-1, got %s", r.Stats[0].Name)
				}
			},
		},
		{
			name: "query specific coin type with no burns",
			cmd: &types.GetBurnedCoinsCmd{
				CoinType: uint8Ptr(2),
			},
			burnedAmounts: map[cointype.CoinType]int64{
				1: 100000000000, // SKA-1 has burns, but we're querying SKA-2
			},
			wantErr:      false,
			wantStatsLen: 0, // No burns for SKA-2, so empty array
			validate: func(t *testing.T, result interface{}) {
				r := result.(types.GetBurnedCoinsResult)
				if len(r.Stats) != 0 {
					t.Errorf("expected 0 stats (no burns), got %d", len(r.Stats))
				}
			},
		},
		{
			name: "query all coin types",
			cmd: &types.GetBurnedCoinsCmd{
				CoinType: nil, // nil means all
			},
			burnedAmounts: map[cointype.CoinType]int64{
				1: 100000000000, // 1000 SKA-1
				2: 50000000000,  // 500 SKA-2
			},
			wantErr:      false,
			wantStatsLen: 2,
			validate: func(t *testing.T, result interface{}) {
				r := result.(types.GetBurnedCoinsResult)
				if len(r.Stats) != 2 {
					t.Errorf("expected 2 stats, got %d", len(r.Stats))
					return
				}
				// Verify both coin types are present (order not guaranteed)
				found1, found2 := false, false
				for _, stat := range r.Stats {
					if stat.CoinType == 1 {
						found1 = true
						if stat.TotalBurned != 1000.0 {
							t.Errorf("SKA-1: expected 1000.0, got %f", stat.TotalBurned)
						}
					}
					if stat.CoinType == 2 {
						found2 = true
						if stat.TotalBurned != 500.0 {
							t.Errorf("SKA-2: expected 500.0, got %f", stat.TotalBurned)
						}
					}
				}
				if !found1 || !found2 {
					t.Errorf("missing coin types in results: found1=%v, found2=%v", found1, found2)
				}
			},
		},
		{
			name: "invalid coin type 0 (VAR)",
			cmd: &types.GetBurnedCoinsCmd{
				CoinType: uint8Ptr(0),
			},
			burnedAmounts: map[cointype.CoinType]int64{},
			wantErr:       true,
		},
		{
			name: "empty burn state",
			cmd: &types.GetBurnedCoinsCmd{
				CoinType: nil,
			},
			burnedAmounts: map[cointype.CoinType]int64{},
			wantErr:       false,
			wantStatsLen:  0,
			validate: func(t *testing.T, result interface{}) {
				r := result.(types.GetBurnedCoinsResult)
				if len(r.Stats) != 0 {
					t.Errorf("expected 0 stats (empty), got %d", len(r.Stats))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create test chain with burn amounts
			mockChain := newTestRPCChainWithBurns(tt.burnedAmounts)

			// Create server with mock chain
			s := &Server{
				cfg: Config{
					Chain: mockChain,
				},
			}

			// Call handler
			ctx := context.Background()
			result, err := handleGetBurnedCoins(ctx, s, tt.cmd)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("handleGetBurnedCoins() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If error expected, we're done
			if tt.wantErr {
				return
			}

			// Validate result length if specified
			if tt.wantStatsLen > 0 {
				r := result.(types.GetBurnedCoinsResult)
				if len(r.Stats) != tt.wantStatsLen {
					t.Errorf("expected %d stats, got %d", tt.wantStatsLen, len(r.Stats))
				}
			}

			// Run custom validation if provided
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestBurnCoinTypeValidation tests coin type validation in burn context.
func TestBurnCoinTypeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		coinType    uint8
		expectValid bool
	}{
		{
			name:        "valid SKA-1",
			coinType:    1,
			expectValid: true,
		},
		{
			name:        "valid SKA-255",
			coinType:    255,
			expectValid: true,
		},
		{
			name:        "invalid VAR (0)",
			coinType:    0,
			expectValid: false,
		},
		{
			name:        "valid SKA-128",
			coinType:    128,
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ct := cointype.CoinType(tt.coinType)
			isValid := ct.IsSKA()

			if isValid != tt.expectValid {
				t.Errorf("CoinType(%d).IsSKA() = %v, want %v",
					tt.coinType, isValid, tt.expectValid)
			}
		})
	}
}

// uint8Ptr returns a pointer to a uint8 value.
func uint8Ptr(v uint8) *uint8 {
	return &v
}
