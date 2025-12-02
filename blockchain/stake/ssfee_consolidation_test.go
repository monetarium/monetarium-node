// Copyright (c) 2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package stake

import (
	"bytes"
	"testing"

	"github.com/decred/dcrd/blockchain/standalone/v2"
	"github.com/decred/dcrd/wire"
)

// TestExtractSSFeeConsolidationAddr tests extraction of consolidation address
// from vote transactions in various scenarios.
func TestExtractSSFeeConsolidationAddr(t *testing.T) {
	// Sample hash160 for testing
	testHash160 := []byte{
		0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
		0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x9a, 0x0b,
	}

	tests := []struct {
		name      string
		buildTx   func() *wire.MsgTx
		wantHash  []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid vote with consolidation at position 2",
			buildTx: func() *wire.MsgTx {
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)}, // [0] block reference
						{PkScript: make([]byte, 4)},  // [1] vote bits
						// [2] consolidation address
						{PkScript: buildConsolidationScript(testHash160)},
					},
				}
				return tx
			},
			wantHash: testHash160,
			wantErr:  false,
		},
		{
			name: "valid vote with consolidation at position 3 (after 1 reward)",
			buildTx: func() *wire.MsgTx {
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)}, // [0] block reference
						{PkScript: make([]byte, 4)},  // [1] vote bits
						{PkScript: make([]byte, 25)}, // [2] reward output (P2PKH)
						// [3] consolidation address
						{PkScript: buildConsolidationScript(testHash160)},
					},
				}
				return tx
			},
			wantHash: testHash160,
			wantErr:  false,
		},
		{
			name: "valid vote with consolidation and treasury vote",
			buildTx: func() *wire.MsgTx {
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)}, // [0] block reference
						{PkScript: make([]byte, 4)},  // [1] vote bits
						{PkScript: make([]byte, 25)}, // [2] reward output
						// [3] consolidation address
						{PkScript: buildConsolidationScript(testHash160)},
						// [4] treasury vote (should be ignored)
						{PkScript: buildTreasuryVoteScript()},
					},
				}
				return tx
			},
			wantHash: testHash160,
			wantErr:  false,
		},
		{
			name: "valid vote with multiple rewards before consolidation",
			buildTx: func() *wire.MsgTx {
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)}, // [0] block reference
						{PkScript: make([]byte, 4)},  // [1] vote bits
						{PkScript: make([]byte, 25)}, // [2] reward 1
						{PkScript: make([]byte, 25)}, // [3] reward 2
						{PkScript: make([]byte, 25)}, // [4] reward 3
						// [5] consolidation address
						{PkScript: buildConsolidationScript(testHash160)},
					},
				}
				return tx
			},
			wantHash: testHash160,
			wantErr:  false,
		},
		{
			name: "missing consolidation output",
			buildTx: func() *wire.MsgTx {
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)}, // [0] block reference
						{PkScript: make([]byte, 4)},  // [1] vote bits
						{PkScript: make([]byte, 25)}, // [2] reward output
					},
				}
				return tx
			},
			wantErr:   true,
			errSubstr: "missing consolidation address",
		},
		{
			name: "too few outputs",
			buildTx: func() *wire.MsgTx {
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)}, // [0] block reference
						{PkScript: make([]byte, 4)},  // [1] vote bits
					},
				}
				return tx
			},
			wantErr:   true,
			errSubstr: "too few outputs",
		},
		{
			name: "invalid marker (wrong first byte)",
			buildTx: func() *wire.MsgTx {
				invalidScript := buildConsolidationScript(testHash160)
				invalidScript[2] = 0x99 // Change 'S' to invalid byte
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)},
						{PkScript: make([]byte, 4)},
						{PkScript: invalidScript},
					},
				}
				return tx
			},
			wantErr:   true,
			errSubstr: "missing consolidation address",
		},
		{
			name: "invalid marker (wrong second byte)",
			buildTx: func() *wire.MsgTx {
				invalidScript := buildConsolidationScript(testHash160)
				invalidScript[3] = 0x99 // Change 'C' to invalid byte
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)},
						{PkScript: make([]byte, 4)},
						{PkScript: invalidScript},
					},
				}
				return tx
			},
			wantErr:   true,
			errSubstr: "missing consolidation address",
		},
		{
			name: "invalid length (too short)",
			buildTx: func() *wire.MsgTx {
				invalidScript := buildConsolidationScript(testHash160)[:23]
				tx := &wire.MsgTx{
					TxOut: []*wire.TxOut{
						{PkScript: make([]byte, 38)},
						{PkScript: make([]byte, 4)},
						{PkScript: invalidScript},
					},
				}
				return tx
			},
			wantErr:   true,
			errSubstr: "missing consolidation address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := tt.buildTx()
			hash160, err := ExtractSSFeeConsolidationAddr(tx)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errSubstr)) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !bytes.Equal(hash160, tt.wantHash) {
				t.Fatalf("hash160 mismatch\nwant: %x\ngot:  %x", tt.wantHash, hash160)
			}

			// Verify hash160 length
			if len(hash160) != 20 {
				t.Fatalf("invalid hash160 length: got %d, want 20", len(hash160))
			}
		})
	}
}

// TestCreateSSFeeConsolidationOutput tests creation of consolidation address outputs.
func TestCreateSSFeeConsolidationOutput(t *testing.T) {
	tests := []struct {
		name      string
		hash160   []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid hash160",
			hash160: []byte{
				0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
				0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x9a, 0x0b,
			},
			wantErr: false,
		},
		{
			name: "all zeros hash160",
			hash160: []byte{
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			wantErr: false,
		},
		{
			name:      "hash160 too short",
			hash160:   make([]byte, 19),
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
		{
			name:      "hash160 too long",
			hash160:   make([]byte, 21),
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
		{
			name:      "nil hash160",
			hash160:   nil,
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := CreateSSFeeConsolidationOutput(tt.hash160)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errSubstr)) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify output properties
			if output.Value != 0 {
				t.Fatalf("expected value=0, got %d", output.Value)
			}

			if output.Version != 0 {
				t.Fatalf("expected version=0, got %d", output.Version)
			}

			// Verify script length
			if len(output.PkScript) != SSConsolidationOutputSize {
				t.Fatalf("expected script length %d, got %d",
					SSConsolidationOutputSize, len(output.PkScript))
			}

			// Verify script format: OP_RETURN + OP_DATA_22 + "SC" + hash160
			script := output.PkScript

			if script[0] != standalone.SSFeeOpReturn {
				t.Fatalf("expected OP_RETURN (0x%x), got 0x%x", standalone.SSFeeOpReturn, script[0])
			}

			if script[1] != SSConsolidationOpData22 {
				t.Fatalf("expected OP_DATA_22 (0x%x), got 0x%x", SSConsolidationOpData22, script[1])
			}

			if script[2] != SSConsolidationMarkerS {
				t.Fatalf("expected 'S' (0x%x), got 0x%x", SSConsolidationMarkerS, script[2])
			}

			if script[3] != SSConsolidationMarkerC {
				t.Fatalf("expected 'C' (0x%x), got 0x%x", SSConsolidationMarkerC, script[3])
			}

			// Verify hash160 is correctly embedded
			if !bytes.Equal(script[4:24], tt.hash160) {
				t.Fatalf("hash160 mismatch\nwant: %x\ngot:  %x", tt.hash160, script[4:24])
			}
		})
	}
}

// TestConsolidationAddrToPkScript tests conversion of hash160 to P2PKH script.
func TestConsolidationAddrToPkScript(t *testing.T) {
	tests := []struct {
		name      string
		hash160   []byte
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid hash160",
			hash160: []byte{
				0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
				0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x9a, 0x0b,
			},
			wantErr: false,
		},
		{
			name:      "hash160 too short",
			hash160:   make([]byte, 19),
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
		{
			name:      "hash160 too long",
			hash160:   make([]byte, 21),
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
		{
			name:      "nil hash160",
			hash160:   nil,
			wantErr:   true,
			errSubstr: "invalid hash160 length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkScript, err := ConsolidationAddrToPkScript(tt.hash160)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errSubstr)) {
					t.Fatalf("expected error containing %q, got %q", tt.errSubstr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify script length (OP_SSGEN-tagged P2PKH is 26 bytes)
			if len(pkScript) != 26 {
				t.Fatalf("expected script length 26, got %d", len(pkScript))
			}

			// Verify OP_SSGEN-tagged P2PKH format:
			// OP_SSGEN OP_DUP OP_HASH160 OP_DATA_20 <hash160> OP_EQUALVERIFY OP_CHECKSIG
			if pkScript[0] != 0xbb { // OP_SSGEN
				t.Fatalf("expected OP_SSGEN (0xbb), got 0x%x", pkScript[0])
			}

			if pkScript[1] != 0x76 { // OP_DUP
				t.Fatalf("expected OP_DUP (0x76), got 0x%x", pkScript[1])
			}

			if pkScript[2] != 0xa9 { // OP_HASH160
				t.Fatalf("expected OP_HASH160 (0xa9), got 0x%x", pkScript[2])
			}

			if pkScript[3] != 0x14 { // OP_DATA_20
				t.Fatalf("expected OP_DATA_20 (0x14), got 0x%x", pkScript[3])
			}

			// Verify hash160
			if !bytes.Equal(pkScript[4:24], tt.hash160) {
				t.Fatalf("hash160 mismatch\nwant: %x\ngot:  %x", tt.hash160, pkScript[4:24])
			}

			if pkScript[24] != 0x88 { // OP_EQUALVERIFY
				t.Fatalf("expected OP_EQUALVERIFY (0x88), got 0x%x", pkScript[24])
			}

			if pkScript[25] != 0xac { // OP_CHECKSIG
				t.Fatalf("expected OP_CHECKSIG (0xac), got 0x%x", pkScript[25])
			}
		})
	}
}

// TestConsolidationRoundtrip tests that Create + Extract produces the original hash160.
func TestConsolidationRoundtrip(t *testing.T) {
	testHash160 := []byte{
		0x1a, 0x2b, 0x3c, 0x4d, 0x5e, 0x6f, 0x7a, 0x8b, 0x9c, 0x0d,
		0x1e, 0x2f, 0x3a, 0x4b, 0x5c, 0x6d, 0x7e, 0x8f, 0x9a, 0x0b,
	}

	// Create output
	output, err := CreateSSFeeConsolidationOutput(testHash160)
	if err != nil {
		t.Fatalf("CreateSSFeeConsolidationOutput failed: %v", err)
	}

	// Build a minimal vote transaction
	tx := &wire.MsgTx{
		TxOut: []*wire.TxOut{
			{PkScript: make([]byte, 38)}, // [0] block reference
			{PkScript: make([]byte, 4)},  // [1] vote bits
			output,                       // [2] consolidation
		},
	}

	// Extract hash160
	extractedHash160, err := ExtractSSFeeConsolidationAddr(tx)
	if err != nil {
		t.Fatalf("ExtractSSFeeConsolidationAddr failed: %v", err)
	}

	// Verify roundtrip
	if !bytes.Equal(extractedHash160, testHash160) {
		t.Fatalf("roundtrip failed\noriginal: %x\nextracted: %x",
			testHash160, extractedHash160)
	}
}

// -----------------------------------------------------------------------------
// Helper functions for building test data
// -----------------------------------------------------------------------------

// buildConsolidationScript creates a valid consolidation address script.
func buildConsolidationScript(hash160 []byte) []byte {
	script := make([]byte, SSConsolidationOutputSize)
	script[0] = standalone.SSFeeOpReturn
	script[1] = SSConsolidationOpData22
	script[2] = SSConsolidationMarkerS
	script[3] = SSConsolidationMarkerC
	copy(script[4:24], hash160)
	return script
}

// buildTreasuryVoteScript creates a minimal treasury vote script for testing.
// Format: OP_RETURN + OP_DATA_N + 'T' + 'V' + ...
func buildTreasuryVoteScript() []byte {
	// Minimal treasury vote: OP_RETURN + OP_DATA_4 + "TV" + 0x00 + 0x00
	return []byte{
		standalone.SSFeeOpReturn, // OP_RETURN
		0x04,                     // OP_DATA_4
		'T', 'V',                 // Treasury vote marker
		0x00, 0x00, // Padding
	}
}
