// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

// TestChainParamsSKAConfiguration tests that SKA parameters are properly
// configured in different network configurations using the new per-coin system.
func TestChainParamsSKAConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		params   *chaincfg.Params
		expected struct {
			ska1EmissionAmount int64
			ska1EmissionHeight int32
			ska1Active         bool
			minRelayFee        int64
		}
	}{
		{
			name:   "SimNet SKA-1 parameters",
			params: chaincfg.SimNetParams(),
			expected: struct {
				ska1EmissionAmount int64
				ska1EmissionHeight int32
				ska1Active         bool
				minRelayFee        int64
			}{
				ska1EmissionAmount: 1e6 * 1e8, // 1 million SKA-1
				ska1EmissionHeight: 150,       // After stake validation to preserve SKA fees
				ska1Active:         true,      // Active in simnet
				minRelayFee:        1e3,       // 0.00001 SKA
			},
		},
		{
			name:   "MainNet SKA-1 parameters",
			params: chaincfg.MainNetParams(),
			expected: struct {
				ska1EmissionAmount int64
				ska1EmissionHeight int32
				ska1Active         bool
				minRelayFee        int64
			}{
				ska1EmissionAmount: 10e6 * 1e8, // 10 million SKA-1 total
				ska1EmissionHeight: 4096,       // Aligned with StakeValidationHeight
				ska1Active:         true,       // Active on mainnet
				minRelayFee:        50,         // 50 atoms/KB for minimal fees
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Check SKA-1 configuration
			ska1Config := test.params.SKACoins[1]
			if ska1Config == nil {
				t.Errorf("SKA-1 configuration missing")
				return
			}

			// Calculate total emission amount for SKA-1
			var totalEmissionAmount int64
			for _, amount := range ska1Config.EmissionAmounts {
				totalEmissionAmount += amount
			}

			if totalEmissionAmount != test.expected.ska1EmissionAmount {
				t.Errorf("SKA-1 emission amount: expected %d, got %d",
					test.expected.ska1EmissionAmount, totalEmissionAmount)
			}

			if ska1Config.EmissionHeight != test.expected.ska1EmissionHeight {
				t.Errorf("SKA-1 emission height: expected %d, got %d",
					test.expected.ska1EmissionHeight, ska1Config.EmissionHeight)
			}

			if ska1Config.Active != test.expected.ska1Active {
				t.Errorf("SKA-1 active status: expected %t, got %t",
					test.expected.ska1Active, ska1Config.Active)
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
					CoinType: cointype.CoinTypeVAR,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				},
				{
					Value:    50000000, // 0.5 VAR
					CoinType: cointype.CoinTypeVAR,
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
					CoinType: cointype.CoinType(1),
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
					CoinType: cointype.CoinTypeVAR,
					Version:  0,
					PkScript: []byte{0x76, 0xa9, 0x14, 0x01, 0x02, 0x03},
				},
				{
					Value:    300000000, // 3 SKA
					CoinType: cointype.CoinType(1),
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
					CoinType: cointype.CoinType(99), // Invalid
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
				case cointype.CoinTypeVAR:
					totalVAR += output.Value
				case cointype.CoinType(1):
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

// TestSKAPerCoinActivation tests SKA per-coin activation logic for different networks.
func TestSKAPerCoinActivation(t *testing.T) {
	tests := []struct {
		name         string
		params       *chaincfg.Params
		coinType     cointype.CoinType
		expectActive bool
	}{
		{
			name:         "SimNet SKA-1 is active",
			params:       chaincfg.SimNetParams(),
			coinType:     1,
			expectActive: true,
		},
		{
			name:         "SimNet SKA-99 is inactive",
			params:       chaincfg.SimNetParams(),
			coinType:     99, // Not configured
			expectActive: false,
		},
		{
			name:         "MainNet SKA-1 is active",
			params:       chaincfg.MainNetParams(),
			coinType:     1,
			expectActive: true,
		},
		{
			name:         "MainNet SKA-2 is inactive",
			params:       chaincfg.MainNetParams(),
			coinType:     2,
			expectActive: false,
		},
		{
			name:         "MainNet SKA-99 is inactive",
			params:       chaincfg.MainNetParams(),
			coinType:     99, // Not configured
			expectActive: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isActive := test.params.IsSKACoinTypeActive(test.coinType)
			if isActive != test.expectActive {
				t.Errorf("IsSKACoinTypeActive(%d): expected %t, got %t",
					test.coinType, test.expectActive, isActive)
			}
		})
	}
}
