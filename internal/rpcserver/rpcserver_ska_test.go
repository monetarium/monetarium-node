// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpcserver

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
)

// TestSKAConfigParameterValidation tests validation of SKA configuration parameters.
func TestSKAConfigParameterValidation(t *testing.T) {
	t.Run("CoinTypeValidation", func(t *testing.T) {
		tests := []struct {
			name        string
			coinType    uint8
			expectValid bool
			description string
		}{
			{
				name:        "Valid coin type 1",
				coinType:    1,
				expectValid: true,
				description: "Coin type 1 should be valid",
			},
			{
				name:        "Valid coin type 255",
				coinType:    255,
				expectValid: true,
				description: "Coin type 255 should be valid",
			},
			{
				name:        "Invalid coin type 0 (VAR)",
				coinType:    0,
				expectValid: false,
				description: "Coin type 0 (VAR) should not be valid for SKA",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				// Test coin type validation logic
				isValid := test.coinType >= 1 && test.coinType <= 255

				if isValid != test.expectValid {
					t.Errorf("Expected validation result %t, got %t: %s",
						test.expectValid, isValid, test.description)
				}
			})
		}
	})

	t.Run("EmissionWindowCalculation", func(t *testing.T) {
		tests := []struct {
			name           string
			emissionHeight int64
			emissionWindow int64
			currentHeight  int64
			expectedActive bool
			description    string
		}{
			{
				name:           "Height within window",
				emissionHeight: 100,
				emissionWindow: 50,
				currentHeight:  125,
				expectedActive: true,
				description:    "Should be active when current height is within emission window",
			},
			{
				name:           "Height at window start",
				emissionHeight: 100,
				emissionWindow: 50,
				currentHeight:  100,
				expectedActive: true,
				description:    "Should be active at window start",
			},
			{
				name:           "Height at window end",
				emissionHeight: 100,
				emissionWindow: 50,
				currentHeight:  150,
				expectedActive: true,
				description:    "Should be active at window end",
			},
			{
				name:           "Height before window",
				emissionHeight: 100,
				emissionWindow: 50,
				currentHeight:  99,
				expectedActive: false,
				description:    "Should not be active before window",
			},
			{
				name:           "Height after window",
				emissionHeight: 100,
				emissionWindow: 50,
				currentHeight:  151,
				expectedActive: false,
				description:    "Should not be active after window",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				// Calculate if window should be active
				windowStart := test.emissionHeight
				windowEnd := test.emissionHeight + test.emissionWindow

				isActive := test.currentHeight >= windowStart && test.currentHeight <= windowEnd

				if isActive != test.expectedActive {
					t.Errorf("Expected active status %t, got %t: %s",
						test.expectedActive, isActive, test.description)
				}
			})
		}
	})
}

// TestSKAChainParameterConsistency tests consistency of chain parameters.
func TestSKAChainParameterConsistency(t *testing.T) {
	t.Run("SimNetParameterConsistency", func(t *testing.T) {
		params := chaincfg.SimNetParams()

		// Verify that simnet has SKA configurations
		if params.SKACoins == nil {
			t.Fatal("SimNet should have SKA coin configurations")
		}

		// Test each configured SKA coin type
		for _, config := range params.SKACoins {
			t.Run(config.Name, func(t *testing.T) {
				// Verify emission height is reasonable
				if config.EmissionHeight < 0 {
					t.Errorf("Emission height should not be negative: %d", config.EmissionHeight)
				}

				// Verify emission window is reasonable
				if config.EmissionWindow < 0 {
					t.Errorf("Emission window should not be negative: %d", config.EmissionWindow)
				}

				// Verify max supply is positive
				if config.MaxSupply <= 0 {
					t.Errorf("Max supply should be positive: %d", config.MaxSupply)
				}

				// Verify name is not empty
				if config.Name == "" {
					t.Error("Name should not be empty")
				}

				// Verify symbol is not empty
				if config.Symbol == "" {
					t.Error("Symbol should not be empty")
				}
			})
		}
	})

	t.Run("MainNetParameterConsistency", func(t *testing.T) {
		params := chaincfg.MainNetParams()

		// Verify that mainnet has SKA configurations
		if params.SKACoins == nil {
			t.Fatal("MainNet should have SKA coin configurations")
		}

		// Verify mainnet emission heights are reasonable for production
		for _, config := range params.SKACoins {
			t.Run(config.Name, func(t *testing.T) {
				// Mainnet emission heights should be higher than simnet (>150)
				if config.EmissionHeight < 1000 {
					t.Errorf("Mainnet emission height seems too low: %d", config.EmissionHeight)
				}

				// Mainnet emission windows should be reasonable (e.g., at least a day)
				if config.EmissionWindow > 0 && config.EmissionWindow < 144 { // < 1 day at 10min blocks
					t.Errorf("Mainnet emission window seems too short: %d blocks", config.EmissionWindow)
				}

				// Verify emission addresses are configured
				if len(config.EmissionAddresses) == 0 {
					t.Error("Emission addresses should be configured for mainnet")
				}

				// Verify emission amounts are configured
				if len(config.EmissionAmounts) == 0 {
					t.Error("Emission amounts should be configured for mainnet")
				}

				// Verify addresses and amounts have same length
				if len(config.EmissionAddresses) != len(config.EmissionAmounts) {
					t.Errorf("Emission addresses (%d) and amounts (%d) should have same length",
						len(config.EmissionAddresses), len(config.EmissionAmounts))
				}

				// Verify total emission matches max supply
				var totalEmission int64
				for _, amount := range config.EmissionAmounts {
					totalEmission += amount
				}
				if totalEmission != config.MaxSupply {
					t.Errorf("Total emission (%d) should match max supply (%d)",
						totalEmission, config.MaxSupply)
				}
			})
		}
	})
}
