// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/txscript/stdaddr"
	"github.com/monetarium/monetarium-node/wire"
)

// TestCreateMinerSSFeeTx tests the createMinerSSFeeTx function.
func TestCreateMinerSSFeeTx(t *testing.T) {
	// Create a mock address for testing - use a simple P2PKH address
	simNetParams := chaincfg.SimNetParams()
	// Create a mock hash for the address
	hash := [20]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14}
	mockAddr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(hash[:], simNetParams)
	if err != nil {
		t.Fatalf("Failed to create mock address: %v", err)
	}

	tests := []struct {
		name         string
		coinType     cointype.CoinType
		totalFee     int64
		minerAddress stdaddr.Address
		height       int64
		expectError  bool
		validateTx   func(*testing.T, *wire.MsgTx)
	}{
		{
			name:         "Valid SKA-1 miner fee",
			coinType:     cointype.CoinType(1),
			totalFee:     100000,
			minerAddress: mockAddr,
			height:       1000,
			expectError:  false,
			validateTx: func(t *testing.T, tx *wire.MsgTx) {
				// Should have exactly 1 input (null input)
				if len(tx.TxIn) != 1 {
					t.Errorf("Expected 1 input, got %d", len(tx.TxIn))
				}

				// Should have exactly 2 outputs (payment + OP_RETURN)
				if len(tx.TxOut) != 2 {
					t.Errorf("Expected 2 outputs, got %d", len(tx.TxOut))
				}

				// First output should be OP_RETURN - standard SSFee format
				// SKA OP_RETURN uses SKAValue (should be 0)
				if tx.TxOut[0].SKAValue == nil || tx.TxOut[0].SKAValue.Sign() != 0 {
					t.Errorf("Expected OP_RETURN SKAValue 0, got %v", tx.TxOut[0].SKAValue)
				}
				if tx.TxOut[0].CoinType != cointype.CoinType(1) {
					t.Errorf("Expected coin type 1, got %d", tx.TxOut[0].CoinType)
				}

				// Second output should be payment - standard SSFee format
				// SKA outputs use SKAValue, not Value
				if tx.TxOut[1].SKAValue == nil || tx.TxOut[1].SKAValue.Int64() != 100000 {
					var got int64
					if tx.TxOut[1].SKAValue != nil {
						got = tx.TxOut[1].SKAValue.Int64()
					}
					t.Errorf("Expected payment SKAValue 100000, got %d", got)
				}

				// Check version
				if tx.Version != 3 {
					t.Errorf("Expected version 3, got %d", tx.Version)
				}
			},
		},
		{
			name:         "Valid SKA-2 miner fee",
			coinType:     cointype.CoinType(2),
			totalFee:     50000,
			minerAddress: mockAddr,
			height:       2000,
			expectError:  false,
			validateTx: func(t *testing.T, tx *wire.MsgTx) {
				// SKA OP_RETURN uses SKAValue (should be 0)
				if tx.TxOut[0].SKAValue == nil || tx.TxOut[0].SKAValue.Sign() != 0 {
					t.Errorf("Expected OP_RETURN SKAValue 0, got %v", tx.TxOut[0].SKAValue)
				}
				if tx.TxOut[0].CoinType != cointype.CoinType(2) {
					t.Errorf("Expected coin type 2, got %d", tx.TxOut[0].CoinType)
				}
			},
		},
		{
			name:         "Invalid VAR coin type",
			coinType:     cointype.CoinTypeVAR,
			totalFee:     100000,
			minerAddress: mockAddr,
			height:       1000,
			expectError:  true,
		},
		{
			name:         "Invalid zero fee",
			coinType:     cointype.CoinType(1),
			totalFee:     0,
			minerAddress: mockAddr,
			height:       1000,
			expectError:  true,
		},
		{
			name:         "Invalid negative fee",
			coinType:     cointype.CoinType(1),
			totalFee:     -100000,
			minerAddress: mockAddr,
			height:       1000,
			expectError:  true,
		},
		{
			name:         "Nil miner address (anyone-can-spend)",
			coinType:     cointype.CoinType(1),
			totalFee:     100000,
			minerAddress: nil,
			height:       1000,
			expectError:  false,
			validateTx: func(t *testing.T, tx *wire.MsgTx) {
				// Second output (payment) should use OP_SSGEN + OP_TRUE script (anyone-can-spend)
				// OP_SSGEN (0xbb) + OP_TRUE (0x51)
				if len(tx.TxOut[1].PkScript) != 2 || tx.TxOut[1].PkScript[0] != 0xbb || tx.TxOut[1].PkScript[1] != 0x51 {
					t.Errorf("Expected OP_SSGEN + OP_TRUE script for nil address, got %x", tx.TxOut[1].PkScript)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Pass nil SSFeeIndex, blockUtxos, fetchUtxoEntry, and generator to test null-input SSFee creation (no augmentation)
			minerSSFeeTx, err := createMinerSSFeeTx(test.coinType, test.totalFee,
				test.minerAddress, test.height, nil, nil, nil, nil)

			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if minerSSFeeTx == nil {
				t.Error("Expected transaction but got nil")
				return
			}

			// Check that transaction is in stake tree
			if minerSSFeeTx.Tree() != wire.TxTreeStake {
				t.Error("Expected transaction to be in stake tree")
			}

			// Run custom validation if provided
			if test.validateTx != nil {
				test.validateTx(t, minerSSFeeTx.MsgTx())
			}
		})
	}
}

// TestMinerSSFeeOpReturn tests that the OP_RETURN output is correctly formatted.
func TestMinerSSFeeOpReturn(t *testing.T) {
	simNetParams := chaincfg.SimNetParams()
	hash := [20]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14}
	mockAddr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(hash[:], simNetParams)
	if err != nil {
		t.Fatalf("Failed to create mock address: %v", err)
	}

	height := int64(12345)
	// Pass nil SSFeeIndex, blockUtxos, fetchUtxoEntry, and generator to test null-input SSFee creation (no augmentation)
	minerSSFeeTx, err := createMinerSSFeeTx(cointype.CoinType(1), 100000, mockAddr, height, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create miner SSFee tx: %v", err)
	}

	// Find OP_RETURN output (should be last output)
	tx := minerSSFeeTx.MsgTx()
	if len(tx.TxOut) < 2 {
		t.Fatal("Expected at least 2 outputs")
	}

	// OP_RETURN should be at index 0 (standard SSFee format)
	opReturnOut := tx.TxOut[0]
	// SKA OP_RETURN uses SKAValue (should be 0)
	if opReturnOut.SKAValue == nil || opReturnOut.SKAValue.Sign() != 0 {
		t.Errorf("OP_RETURN output should have SKAValue 0, got %v", opReturnOut.SKAValue)
	}

	// Check OP_RETURN script format
	script := opReturnOut.PkScript
	if len(script) < 8 {
		t.Fatalf("OP_RETURN script too short: %d bytes", len(script))
	}

	// Should start with OP_RETURN (0x6a) and OP_DATA_6 (0x06)
	if script[0] != 0x6a || script[1] != 0x06 {
		t.Errorf("Invalid OP_RETURN format: %x %x", script[0], script[1])
	}

	// Should contain "MF" marker
	if script[2] != 'M' || script[3] != 'F' {
		t.Errorf("Expected 'MF' marker, got '%c%c'", script[2], script[3])
	}

	// Check height encoding (little-endian uint32)
	encodedHeight := uint32(script[4]) |
		uint32(script[5])<<8 |
		uint32(script[6])<<16 |
		uint32(script[7])<<24

	if encodedHeight != uint32(height) {
		t.Errorf("Expected height %d, got %d", height, encodedHeight)
	}
}

// TestMinerSSFeeDistribution tests that miner SSFee correctly distributes fees.
func TestMinerSSFeeDistribution(t *testing.T) {
	simNetParams := chaincfg.SimNetParams()
	hash := [20]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a,
		0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14}
	minerAddr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(hash[:], simNetParams)
	if err != nil {
		t.Fatalf("Failed to create miner address: %v", err)
	}

	// Test different fee amounts for different coin types
	testCases := []struct {
		coinType  cointype.CoinType
		feeAmount int64
	}{
		{cointype.CoinType(1), 1000000}, // 0.01 SKA-1
		{cointype.CoinType(2), 500000},  // 0.005 SKA-2
		{cointype.CoinType(3), 250000},  // 0.0025 SKA-3
	}

	for _, tc := range testCases {
		// Pass nil SSFeeIndex, blockUtxos, fetchUtxoEntry, and generator to test null-input SSFee creation (no augmentation)
		minerSSFeeTx, err := createMinerSSFeeTx(tc.coinType, tc.feeAmount, minerAddr, 1000, nil, nil, nil, nil)
		if err != nil {
			t.Errorf("Failed to create miner SSFee for coin type %d: %v", tc.coinType, err)
			continue
		}

		tx := minerSSFeeTx.MsgTx()

		// Verify input value matches fee amount
		// For SKA SSFee, input uses SKAValueIn (big.Int), not ValueIn
		var inputValue int64
		if tc.coinType.IsSKA() && tx.TxIn[0].SKAValueIn != nil {
			inputValue = tx.TxIn[0].SKAValueIn.Int64()
		} else {
			inputValue = tx.TxIn[0].ValueIn
		}
		if inputValue != tc.feeAmount {
			t.Errorf("Input value mismatch for coin type %d: expected %d, got %d",
				tc.coinType, tc.feeAmount, inputValue)
		}

		// Verify payment output value matches fee amount (output[1] in standard SSFee format)
		// SKA outputs use SKAValue, not Value
		var outputValue int64
		if tx.TxOut[1].SKAValue != nil {
			outputValue = tx.TxOut[1].SKAValue.Int64()
		}
		if outputValue != tc.feeAmount {
			t.Errorf("Output value mismatch for coin type %d: expected %d, got %d",
				tc.coinType, tc.feeAmount, outputValue)
		}

		// Verify coin type is correct
		if tx.TxOut[1].CoinType != tc.coinType {
			t.Errorf("Output coin type mismatch: expected %d, got %d",
				tc.coinType, tx.TxOut[1].CoinType)
		}
	}
}
