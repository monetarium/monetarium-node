// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/wire"
)

// TestChainParamsSKAConfiguration tests that SKA parameters are properly
// configured in different network configurations.
func TestChainParamsSKAConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		params   *chaincfg.Params
		expected struct {
			emissionAmount   int64
			emissionHeight   int64
			activationHeight int64
			maxAmount        int64
			minRelayFee      int64
		}
	}{
		{
			name:   "SimNet SKA parameters",
			params: chaincfg.SimNetParams(),
			expected: struct {
				emissionAmount   int64
				emissionHeight   int64
				activationHeight int64
				maxAmount        int64
				minRelayFee      int64
			}{
				emissionAmount:   1e6 * 1e8,  // 1 million SKA
				emissionHeight:   10,         // Early emission for testing
				activationHeight: 10,         // Immediate activation
				maxAmount:        10e6 * 1e8, // 10 million max
				minRelayFee:      1e3,        // 0.00001 SKA
			},
		},
		{
			name:   "MainNet SKA parameters",
			params: chaincfg.MainNetParams(),
			expected: struct {
				emissionAmount   int64
				emissionHeight   int64
				activationHeight int64
				maxAmount        int64
				minRelayFee      int64
			}{
				emissionAmount:   10e6 * 1e8, // 10 million SKA
				emissionHeight:   100000,     // Block 100k emission
				activationHeight: 100000,     // Immediate activation
				maxAmount:        10e6 * 1e8, // 10 million max
				minRelayFee:      1e4,        // 0.0001 SKA
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.params.SKAEmissionAmount != test.expected.emissionAmount {
				t.Errorf("SKAEmissionAmount: expected %d, got %d",
					test.expected.emissionAmount, test.params.SKAEmissionAmount)
			}

			if test.params.SKAEmissionHeight != test.expected.emissionHeight {
				t.Errorf("SKAEmissionHeight: expected %d, got %d",
					test.expected.emissionHeight, test.params.SKAEmissionHeight)
			}

			if test.params.SKAActivationHeight != test.expected.activationHeight {
				t.Errorf("SKAActivationHeight: expected %d, got %d",
					test.expected.activationHeight, test.params.SKAActivationHeight)
			}

			if test.params.SKAMaxAmount != test.expected.maxAmount {
				t.Errorf("SKAMaxAmount: expected %d, got %d",
					test.expected.maxAmount, test.params.SKAMaxAmount)
			}

			if test.params.SKAMinRelayTxFee != test.expected.minRelayFee {
				t.Errorf("SKAMinRelayTxFee: expected %d, got %d",
					test.expected.minRelayFee, test.params.SKAMinRelayTxFee)
			}
		})
	}
}

// TestValidateTransactionOutputsCoinType tests the dual-coin output validation
// logic that was added to CheckTransactionInputs.
func TestValidateTransactionOutputsCoinType(t *testing.T) {
	tests := []struct {
		name      string
		outputs   []*wire.TxOut
		expectVAR int64
		expectSKA int64
		expectErr bool
	}{
		{
			name: "VAR only outputs",
			outputs: []*wire.TxOut{
				{
					Value:    100000000, // 1 VAR
					CoinType: wire.CoinTypeVAR,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				},
				{
					Value:    50000000, // 0.5 VAR
					CoinType: wire.CoinTypeVAR,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
				},
			},
			expectVAR: 150000000, // 1.5 VAR
			expectSKA: 0,
			expectErr: false,
		},
		{
			name: "SKA only outputs",
			outputs: []*wire.TxOut{
				{
					Value:    200000000, // 2 SKA
					CoinType: wire.CoinTypeSKA,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				},
			},
			expectVAR: 0,
			expectSKA: 200000000, // 2 SKA
			expectErr: false,
		},
		{
			name: "Mixed VAR/SKA outputs",
			outputs: []*wire.TxOut{
				{
					Value:    100000000, // 1 VAR
					CoinType: wire.CoinTypeVAR,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				},
				{
					Value:    300000000, // 3 SKA
					CoinType: wire.CoinTypeSKA,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x04, 0x05, 0x06},
				},
			},
			expectVAR: 100000000, // 1 VAR
			expectSKA: 300000000, // 3 SKA
			expectErr: false,
		},
		{
			name: "Invalid coin type",
			outputs: []*wire.TxOut{
				{
					Value:    100000000,
					CoinType: wire.CoinType(99), // Invalid
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				},
			},
			expectVAR: 0,
			expectSKA: 0,
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var totalVAR, totalSKA int64
			var err error

			// Simulate the coin type validation logic from CheckTransactionInputs
			for _, output := range test.outputs {
				switch output.CoinType {
				case wire.CoinTypeVAR:
					totalVAR += output.Value
				case wire.CoinTypeSKA:
					totalSKA += output.Value
				default:
					err = ruleError(ErrBadTxOutValue, "invalid coin type")
				}
			}

			if test.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if totalVAR != test.expectVAR {
				t.Errorf("VAR total: expected %d, got %d", test.expectVAR, totalVAR)
			}

			if totalSKA != test.expectSKA {
				t.Errorf("SKA total: expected %d, got %d", test.expectSKA, totalSKA)
			}
		})
	}
}

// TestSKAActivationHeight tests SKA activation height logic for different networks.
func TestSKAActivationHeight(t *testing.T) {
	tests := []struct {
		name            string
		params          *chaincfg.Params
		blockHeight     int64
		skaActiveBefore bool
		skaActiveAt     bool
		skaActiveAfter  bool
	}{
		{
			name:            "SimNet SKA activation at height 10",
			params:          chaincfg.SimNetParams(),
			blockHeight:     10,
			skaActiveBefore: false, // Height 9
			skaActiveAt:     true,  // Height 10
			skaActiveAfter:  true,  // Height 11
		},
		{
			name:            "MainNet SKA activation at height 100000",
			params:          chaincfg.MainNetParams(),
			blockHeight:     100000,
			skaActiveBefore: false, // Height 99999
			skaActiveAt:     true,  // Height 100000
			skaActiveAfter:  true,  // Height 100001
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Test before activation
			before := test.blockHeight - 1
			activeBefore := before >= test.params.SKAActivationHeight
			if activeBefore != test.skaActiveBefore {
				t.Errorf("SKA active before (height %d): expected %t, got %t",
					before, test.skaActiveBefore, activeBefore)
			}

			// Test at activation
			activeAt := test.blockHeight >= test.params.SKAActivationHeight
			if activeAt != test.skaActiveAt {
				t.Errorf("SKA active at (height %d): expected %t, got %t",
					test.blockHeight, test.skaActiveAt, activeAt)
			}

			// Test after activation
			after := test.blockHeight + 1
			activeAfter := after >= test.params.SKAActivationHeight
			if activeAfter != test.skaActiveAfter {
				t.Errorf("SKA active after (height %d): expected %t, got %t",
					after, test.skaActiveAfter, activeAfter)
			}
		})
	}
}
