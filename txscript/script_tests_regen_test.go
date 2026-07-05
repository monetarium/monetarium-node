// Copyright (c) 2026 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1/ecdsa"
)

// knownTestKeys maps the public key hex strings used by script_tests.json to
// the private key the test data documents (lines 1953–1968 of the file).
// Reference vectors only sign with these three keys.
var knownTestKeys = map[string][]byte{
	// privkey 1 (0x01)
	"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000001"),
	"0479be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000001"),
	"0579be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000001"),
	// privkey 2 (0x03)
	"02f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000003"),
	"04f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9388f7b0f632de8140fe337e62a37f3566500a99934c2231b6cb9fd7584b8e672": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000003"),
	"05f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9388f7b0f632de8140fe337e62a37f3566500a99934c2231b6cb9fd7584b8e672": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000003"),
	// privkey 3 (0x06)
	"03fff97bd5755eeea420453a14355235d382f6472f8568a18b2f057a1460297556": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000006"),
	"04fff97bd5755eeea420453a14355235d382f6472f8568a18b2f057a1460297556ae12777aacfbb620f3be96017f45c560de80f0f6518fe4a03c870c36b075f297": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000006"),
	"06fff97bd5755eeea420453a14355235d382f6472f8568a18b2f057a1460297556ae12777aacfbb620f3be96017f45c560de80f0f6518fe4a03c870c36b075f297": fromHex(
		"0000000000000000000000000000000000000000000000000000000000000006"),
}

func fromHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// extractPubKeysFromScript returns the hex form of every plausible secp256k1
// public key found as a data push in the given parsed script.  Used to
// identify which private key signed a given test entry.  Also recurses one
// level into data pushes large enough to contain a redeem-script pubkey
// push (e.g. P2SH).
func extractPubKeysFromScript(script []byte) []string {
	var out []string
	tokenizer := MakeScriptTokenizer(0, script)
	for tokenizer.Next() {
		data := tokenizer.Data()
		switch len(data) {
		case 33, 65: // compressed / uncompressed / hybrid pubkey push
			out = append(out, hex.EncodeToString(data))
		default:
			// Larger pushes may be a P2SH redeem script; recurse one
			// level so we can pick up the pubkey inside it.
			if len(data) > 33 {
				out = append(out, extractPubKeysFromScript(data)...)
			}
		}
	}
	return out
}

// isP2SHPubKey reports whether the given parsed scriptPubKey matches the
// canonical P2SH form: OP_HASH160 <20-byte-script-hash> OP_EQUAL.
func isP2SHPubKey(script []byte) bool {
	return len(script) == 23 &&
		script[0] == OP_HASH160 &&
		script[1] == 20 &&
		script[22] == OP_EQUAL
}

// lastDataPush returns the bytes of the final data push in script, or nil if
// the script ends in an opcode rather than a push.  Used to extract the
// redeem-script bytes from a P2SH scriptSig.
func lastDataPush(script []byte) []byte {
	var last []byte
	tokenizer := MakeScriptTokenizer(0, script)
	for tokenizer.Next() {
		if data := tokenizer.Data(); len(data) > 0 {
			last = data
		}
	}
	return last
}

// extractSigsFromScript returns each data push of length >= 70 (DER signature
// + 1-byte sighash flag).  These are the signatures a regen pass needs to
// rewrite.
func extractSigsFromScript(script []byte) [][]byte {
	var out [][]byte
	tokenizer := MakeScriptTokenizer(0, script)
	for tokenizer.Next() {
		data := tokenizer.Data()
		if len(data) >= 70 && len(data) <= 73 {
			out = append(out, data)
		}
	}
	return out
}

// rebuildSigScript walks every token in the source script, replacing each
// signature-shaped data push (length 70..73, ends in a single SigHashType
// byte) using the provided replacer.  All non-signature pushes and all
// opcodes are preserved verbatim.  Returns the rebuilt script and a flag
// indicating whether any replacement actually happened.
func rebuildSigScript(script []byte, replace func(sig []byte) []byte) ([]byte, bool, error) {
	builder := NewScriptBuilder()
	changed := false
	tokenizer := MakeScriptTokenizer(0, script)
	for tokenizer.Next() {
		op := tokenizer.Opcode()
		data := tokenizer.Data()
		if len(data) > 0 {
			if len(data) >= 70 && len(data) <= 73 {
				if newSig := replace(data); newSig != nil {
					builder.AddData(newSig)
					changed = true
					continue
				}
			}
			builder.AddData(data)
			continue
		}
		builder.AddOp(op)
	}
	if err := tokenizer.Err(); err != nil {
		return nil, false, err
	}
	rebuilt, err := builder.Script()
	if err != nil {
		return nil, false, err
	}
	return rebuilt, changed, nil
}

// TestRegenerateScriptTestsSignatures rewrites every "CHECKSIG must work" /
// "CHECKMULTISIG must work" / "CHECKSIG must error when ..." entry in
// script_tests.json that signs with a documented private key (1, 3, or 6),
// using the freshly-extended sighash digest.  Skipped unless
// MONETARIUM_REGEN_SCRIPT_TESTS=1 is set.
//
// Negative-result entries (those expecting non-OK errors) are left
// untouched — they intentionally use malformed signatures.
func TestRegenerateScriptTestsSignatures(t *testing.T) {
	if os.Getenv("MONETARIUM_REGEN_SCRIPT_TESTS") != "1" {
		t.Skip("set MONETARIUM_REGEN_SCRIPT_TESTS=1 to regenerate testdata/script_tests.json")
	}

	path := filepath.Join(testDataPath, "script_tests.json")
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read script_tests.json: %v", err)
	}
	var tests [][]string
	if err := json.Unmarshal(file, &tests); err != nil {
		t.Fatalf("unmarshal script_tests.json: %v", err)
	}

	regenerated := 0
	skipped := 0
	for i, test := range tests {
		if len(test) < 4 {
			continue
		}
		// Negative-result tests intentionally use malformed sigs.
		// However, multisig tests like "must error when R is negative
		// (2-of-2, first sig valid DER, second sig valid non-DER)"
		// depend on the *other* (well-formed) sig actually verifying so
		// the loop advances and the malformed sig is reached.  When the
		// digest changes, that incidental verification breaks and the
		// parse error never fires.  We re-sign the well-formed sig only,
		// leaving structurally invalid sigs alone.
		negativeCase := test[3] != "OK"
		sigScriptStr := test[0]
		pubScriptStr := test[1]
		scriptSig, err := parseShortFormV0(sigScriptStr)
		if err != nil {
			continue
		}
		scriptPubKey, err := parseShortFormV0(pubScriptStr)
		if err != nil {
			continue
		}
		sigs := extractSigsFromScript(scriptSig)
		if len(sigs) == 0 {
			continue
		}
		// Determine the subscript the signature is computed over.  For
		// P2SH spends the digest is signed against the redeem script
		// (last data push of scriptSig), not against scriptPubKey.
		subScript := scriptPubKey
		if isP2SHPubKey(scriptPubKey) {
			rs := lastDataPush(scriptSig)
			if rs == nil {
				continue
			}
			subScript = rs
		}

		// Look in the subscript first (where bare CHECKSIG / CHECKMULTISIG
		// directly references pubkeys, and where P2SH redeem scripts
		// embed them).  Fall back to scriptSig for P2PKH, which puts the
		// pubkey alongside the signature push.
		pubKeys := append(
			extractPubKeysFromScript(subScript),
			extractPubKeysFromScript(scriptSig)...,
		)
		if len(pubKeys) == 0 {
			continue
		}
		// We can only regenerate signatures whose pubkey we know the
		// private key for.  If any required key is unknown, skip.
		var keyForPubKey []*secp256k1.PrivateKey
		allKnown := true
		for _, pk := range pubKeys {
			if priv, ok := knownTestKeys[pk]; ok {
				keyForPubKey = append(keyForPubKey, secp256k1.PrivKeyFromBytes(priv))
			} else {
				allKnown = false
				break
			}
		}
		if !allKnown {
			skipped++
			continue
		}
		// In negative-result tests, only re-sign at all when at least one
		// other sig in the same test is structurally malformed.  That
		// pattern (one valid + one parse-error sig) only works if the
		// valid sig actually verifies, so its sighash binding must
		// follow digest changes.  Tests with a single tampered sig
		// (e.g. "sig invalidated via 8th byte ^= 0x55") leave the sig
		// alone — the test is asserting verification failure, not
		// parse failure.
		shouldRegenInNegative := false
		if negativeCase {
			for _, s := range sigs {
				if len(s) < 1 {
					continue
				}
				if err := CheckSignatureEncoding(s[:len(s)-1]); err != nil {
					shouldRegenInNegative = true
					break
				}
			}
			if !shouldRegenInNegative {
				skipped++
				continue
			}
		}

		// Build the spending tx and recompute each signature.
		tx := createSpendingTx(scriptSig, scriptPubKey)
		newSigs := make([][]byte, 0, len(sigs))
		for j, oldSig := range sigs {
			if j >= len(keyForPubKey) {
				break
			}
			if len(oldSig) < 1 {
				newSigs = append(newSigs, oldSig)
				continue
			}
			// In negative-result tests, only re-sign sigs that are
			// already structurally valid; leave intentionally
			// malformed sigs alone so the test still triggers the
			// parse error it was designed to catch.
			if negativeCase {
				if err := CheckSignatureEncoding(oldSig[:len(oldSig)-1]); err != nil {
					newSigs = append(newSigs, oldSig)
					continue
				}
			}
			sigHashType := SigHashType(oldSig[len(oldSig)-1])
			digest, err := calcSignatureHash(subScript, sigHashType, tx, 0, nil)
			if err != nil {
				t.Logf("test #%d (%q): calc digest: %v", i, test[len(test)-1], err)
				newSigs = append(newSigs, oldSig)
				continue
			}
			sig := ecdsa.Sign(keyForPubKey[j], digest)
			der := sig.Serialize()
			fresh := append(append([]byte{}, der...), byte(sigHashType))
			newSigs = append(newSigs, fresh)
		}
		// Rebuild scriptSig as canonical pushes of the new signatures.
		// CHECKMULTISIG entries traditionally include a leading OP_0 for
		// the off-by-one bug, which appears in the original script as a
		// non-data-push opcode.  Preserve any non-signature opcodes by
		// rebuilding via tokenizer.
		newScriptSig, changed, err := rebuildSigScript(scriptSig, func(old []byte) []byte {
			for k, oldSig := range sigs {
				if string(oldSig) == string(old) && k < len(newSigs) {
					return newSigs[k]
				}
			}
			return nil
		})
		if err != nil {
			t.Logf("test #%d (%q): rebuild scriptSig: %v", i, test[len(test)-1], err)
			skipped++
			continue
		}
		if !changed {
			skipped++
			continue
		}
		// Encode the rebuilt scriptSig back into the short form the test
		// data uses: plain hex with "0x" prefix and length byte folded in.
		test[0] = encodeAsShortForm(newScriptSig)
		tests[i] = test
		regenerated++
	}

	// Marshal back, preserving the project's compact one-line-per-entry
	// formatting.
	var out strings.Builder
	out.WriteString("[\n")
	for i, test := range tests {
		entry, err := json.Marshal(test)
		if err != nil {
			t.Fatalf("test #%d: marshal: %v", i, err)
		}
		out.WriteString(string(entry))
		if i < len(tests)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("]\n")

	if err := os.WriteFile(path, []byte(out.String()), 0644); err != nil {
		t.Fatalf("write script_tests.json: %v", err)
	}
	fmt.Printf("regenerated %d entries (skipped %d) in %s\n", regenerated, skipped, path)
}

// encodeAsShortForm encodes a raw script as a single "0xHH 0xHH..." short
// form string so the regenerated entry round-trips through
// parseShortFormV0.  Keeps each chunk reasonably-sized for readability.
func encodeAsShortForm(script []byte) string {
	if len(script) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("0x")
	sb.WriteString(hex.EncodeToString(script))
	return sb.String()
}
