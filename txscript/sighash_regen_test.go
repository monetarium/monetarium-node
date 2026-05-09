// Copyright (c) 2026 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegenerateSighashTestData rewrites testdata/sighash.json with the digest
// produced by the current calcSignatureHash implementation.  Skipped unless
// MONETARIUM_REGEN_SIGHASH=1 is set so it never fires in normal runs.
//
// This exists because Monetarium extended the sighash prefix to commit to
// SKAValueIn / CoinType / SKAValue, so every previously-recorded digest in
// sighash.json is intentionally invalid.  Run once per consensus change
// to refresh the vectors:
//
//	MONETARIUM_REGEN_SIGHASH=1 go test -run TestRegenerateSighashTestData ./txscript/...
func TestRegenerateSighashTestData(t *testing.T) {
	if os.Getenv("MONETARIUM_REGEN_SIGHASH") != "1" {
		t.Skip("set MONETARIUM_REGEN_SIGHASH=1 to regenerate testdata/sighash.json")
	}

	path := filepath.Join(testDataPath, "sighash.json")
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sighash.json: %v", err)
	}
	var tests [][]interface{}
	if err := json.Unmarshal(file, &tests); err != nil {
		t.Fatalf("unmarshal sighash.json: %v", err)
	}

	const scriptVersion = 0
	for i, test := range tests {
		if len(test) == 1 {
			continue
		}
		if len(test) < 6 || len(test) > 7 {
			t.Fatalf("test #%d malformed (len=%d)", i, len(test))
		}
		txHex, _ := test[0].(string)
		rawTx, err := hex.DecodeString(txHex)
		if err != nil {
			t.Fatalf("test #%d: tx hex: %v", i, err)
		}
		tx, err := deserializeTxWithLegacyFallback(rawTx)
		if err != nil {
			if errors.Is(err, errIncompatibleWireFormat) {
				continue
			}
			t.Fatalf("test #%d: deserialize tx: %v", i, err)
		}
		subScriptHex, _ := test[1].(string)
		subScript, err := hex.DecodeString(subScriptHex)
		if err != nil {
			t.Fatalf("test #%d: script hex: %v", i, err)
		}
		if err := checkScriptParses(scriptVersion, subScript); err != nil {
			continue
		}
		idxF64, _ := test[2].(float64)
		hashTypeF64, _ := test[3].(float64)
		expectedErrStr, _ := test[5].(string)
		expectedErr, _ := parseSigHashExpectedResult(expectedErrStr)

		hash, err := calcSignatureHash(subScript, SigHashType(testVecF64ToUint32(hashTypeF64)),
			tx, int(idxF64), nil)
		if !errors.Is(err, expectedErr) {
			// Don't replace the digest if the expected error case still
			// triggers; just leave the original entry alone.
			continue
		}
		if hash == nil {
			continue
		}
		test[4] = hex.EncodeToString(hash)
		tests[i] = test
	}

	// Marshal back, preserving the project's compact one-line-per-entry
	// formatting so diffs stay reviewable.
	var out strings.Builder
	out.WriteString("[\n")
	for i, test := range tests {
		out.WriteString("    ")
		entry, err := json.Marshal(test)
		if err != nil {
			t.Fatalf("test #%d: marshal: %v", i, err)
		}
		// Reformat: opening "[" + newline-indented elements is the
		// existing style; mimic it.
		var fields []json.RawMessage
		if err := json.Unmarshal(entry, &fields); err != nil {
			t.Fatalf("test #%d: re-unmarshal: %v", i, err)
		}
		out.WriteString("[\n")
		for j, field := range fields {
			out.WriteString("        ")
			out.Write(field)
			if j < len(fields)-1 {
				out.WriteString(",")
			}
			out.WriteString("\n")
		}
		out.WriteString("    ]")
		if i < len(tests)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("]\n")

	if err := os.WriteFile(path, []byte(out.String()), 0644); err != nil {
		t.Fatalf("write sighash.json: %v", err)
	}
	fmt.Printf("regenerated %d entries in %s\n", len(tests), path)
}
