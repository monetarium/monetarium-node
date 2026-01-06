//go:build ignore

// This tool regenerates ECDSA signatures in script_tests.json for the new
// wire format (protocol version 12 with CoinType field).
//
// The script tests use a well-known secp256k1 key (generator point G with
// private key = 1) for all signature tests.
//
// Usage: go run regenerate_script_signatures.go

package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1"
	"github.com/monetarium/monetarium-node/dcrec/secp256k1/ecdsa"
	"github.com/monetarium/monetarium-node/txscript"
	"github.com/monetarium/monetarium-node/wire"
)

const (
	scriptTestsFile = "../txscript/testdata/script_tests.json"
)

// Known test private keys (32 bytes, big-endian)
var knownPrivKeys = map[string][]byte{
	// Generator point G (compressed) - private key = 1
	"0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798": {
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	},
	// Generator point G (uncompressed) - same private key = 1
	"0479be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8": {
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	},
	// Compressed with Y bit set (still G, private key = 1)
	"0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798": {
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	},
}

func main() {
	fmt.Println("=== Script Signature Regeneration Tool ===")
	fmt.Println()

	if err := regenerateScriptSignatures(); err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== Signature Regeneration Complete ===")
}

func regenerateScriptSignatures() error {
	fmt.Printf("Processing %s...\n", scriptTestsFile)

	// Read file
	data, err := os.ReadFile(scriptTestsFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// Create backup
	backupPath := scriptTestsFile + ".legacy_v11"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			return fmt.Errorf("failed to create backup: %v", err)
		}
		fmt.Printf("  Created backup: %s\n", backupPath)
	}

	// Parse JSON
	var tests [][]interface{}
	if err := json.Unmarshal(data, &tests); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	updatedCount := 0
	skippedCount := 0

	for i, test := range tests {
		// Skip comments
		if len(test) == 1 {
			continue
		}

		// Format: [scriptSig, scriptPubKey, flags, expectedError, comment]
		if len(test) < 4 {
			continue
		}

		sigScriptStr, ok := test[0].(string)
		if !ok {
			continue
		}

		pkScriptStr, ok := test[1].(string)
		if !ok {
			continue
		}

		expectedResult, ok := test[3].(string)
		if !ok {
			continue
		}

		// Only process tests that are expected to pass and contain signatures
		if expectedResult != "OK" {
			continue
		}

		// Check if this is a signature test (contains CHECKSIG or CHECKMULTISIG)
		if !strings.Contains(pkScriptStr, "CHECKSIG") {
			continue
		}

		// Try to regenerate the signature
		newSigScript, err := regenerateTestSignature(sigScriptStr, pkScriptStr)
		if err != nil {
			// Log but don't fail - some tests may have complex scripts we can't handle
			if strings.Contains(err.Error(), "unknown pubkey") {
				skippedCount++
			} else {
				fmt.Printf("  Warning: test %d: %v\n", i, err)
			}
			continue
		}

		if newSigScript != sigScriptStr {
			tests[i][0] = newSigScript
			updatedCount++
		}
	}

	// Write updated JSON
	output, err := json.MarshalIndent(tests, "", "")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Fix formatting to match original (no indent, arrays on one line)
	output = formatScriptTestsJSON(output)

	if err := os.WriteFile(scriptTestsFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	fmt.Printf("  Updated %d signatures, skipped %d (unknown keys)\n", updatedCount, skippedCount)
	return nil
}

func regenerateTestSignature(sigScriptStr, pkScriptStr string) (string, error) {
	// Parse the pkScript to extract the public key
	pkScript, err := parseShortForm(pkScriptStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse pkScript: %v", err)
	}

	// Extract pubkey from pkScript
	pubKeyBytes, err := extractPubKeyFromScript(pkScript, pkScriptStr)
	if err != nil {
		return "", fmt.Errorf("failed to extract pubkey: %v", err)
	}

	// Get private key
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	privKeyBytes, ok := knownPrivKeys[pubKeyHex]
	if !ok {
		return "", fmt.Errorf("unknown pubkey: %s", pubKeyHex)
	}

	// Parse sigScript to get hash type
	sigScript, err := parseShortForm(sigScriptStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse sigScript: %v", err)
	}

	hashType, err := extractHashType(sigScript)
	if err != nil {
		return "", fmt.Errorf("failed to extract hash type: %v", err)
	}

	// Create test transactions (same as createSpendingTx in reference_test.go)
	coinbaseTx := wire.NewMsgTx()
	outPoint := wire.NewOutPoint(&chainhash.Hash{}, ^uint32(0), wire.TxTreeRegular)
	txIn := wire.NewTxIn(outPoint, 0, []byte{txscript.OP_0, txscript.OP_0})
	txOut := wire.NewTxOut(0, pkScript)
	coinbaseTx.AddTxIn(txIn)
	coinbaseTx.AddTxOut(txOut)

	spendingTx := wire.NewMsgTx()
	coinbaseTxHash := coinbaseTx.TxHash()
	outPoint = wire.NewOutPoint(&coinbaseTxHash, 0, wire.TxTreeRegular)
	txIn = wire.NewTxIn(outPoint, 0, nil) // sigScript will be set
	txOut = wire.NewTxOut(0, nil)
	spendingTx.AddTxIn(txIn)
	spendingTx.AddTxOut(txOut)

	// Calculate signature hash
	sigHash, err := txscript.CalcSignatureHash(pkScript, hashType, spendingTx, 0, nil)
	if err != nil {
		return "", fmt.Errorf("failed to calc sig hash: %v", err)
	}

	// Sign with private key
	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	sig := ecdsa.Sign(privKey, sigHash)

	// Build signature bytes (DER + hash type)
	sigBytes := append(sig.Serialize(), byte(hashType))

	// Build new sigScript
	builder := txscript.NewScriptBuilder()
	builder.AddData(sigBytes)
	builder.AddData(pubKeyBytes)
	newSigScript, err := builder.Script()
	if err != nil {
		return "", fmt.Errorf("failed to build sigScript: %v", err)
	}

	// Convert to short form
	return scriptToShortForm(newSigScript), nil
}

func extractPubKeyFromScript(script []byte, shortForm string) ([]byte, error) {
	// Look for DATA_33 or DATA_65 followed by pubkey hex in short form
	re := regexp.MustCompile(`(?:DATA_33|0x21)\s+0x([0-9a-fA-F]{66})`)
	matches := re.FindStringSubmatch(shortForm)
	if len(matches) > 1 {
		return hex.DecodeString(matches[1])
	}

	re = regexp.MustCompile(`(?:DATA_65|0x41)\s+0x([0-9a-fA-F]{130})`)
	matches = re.FindStringSubmatch(shortForm)
	if len(matches) > 1 {
		return hex.DecodeString(matches[1])
	}

	return nil, fmt.Errorf("no pubkey found in script")
}

func extractHashType(sigScript []byte) (txscript.SigHashType, error) {
	// The hash type is the last byte of the signature
	// Signature is typically the first data push in the script
	if len(sigScript) < 2 {
		return 0, fmt.Errorf("sigScript too short")
	}

	// Get first push length
	pushLen := int(sigScript[0])
	if pushLen < 0x4c { // Direct push
		if len(sigScript) < pushLen+1 {
			return 0, fmt.Errorf("invalid push length")
		}
		// Hash type is last byte of the signature
		return txscript.SigHashType(sigScript[pushLen]), nil
	}

	return txscript.SigHashAll, nil // Default
}

func parseShortForm(s string) ([]byte, error) {
	// This is a simplified parser - the real one is in txscript
	// Handle common patterns in test data

	s = strings.TrimSpace(s)
	if s == "" {
		return []byte{}, nil
	}

	var result []byte
	tokens := strings.Fields(s)

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		// Handle hex data (0x...)
		if strings.HasPrefix(token, "0x") {
			data, err := hex.DecodeString(token[2:])
			if err != nil {
				return nil, fmt.Errorf("invalid hex: %s", token)
			}
			result = append(result, data...)
			continue
		}

		// Handle DATA_N push opcodes
		if strings.HasPrefix(token, "DATA_") {
			lenStr := token[5:]
			pushLen, err := strconv.Atoi(lenStr)
			if err != nil {
				return nil, fmt.Errorf("invalid DATA_ length: %s", token)
			}
			result = append(result, byte(pushLen))
			continue
		}

		// Handle named opcodes
		switch token {
		case "OP_0", "0":
			result = append(result, txscript.OP_0)
		case "OP_1", "1", "TRUE":
			result = append(result, txscript.OP_TRUE)
		case "OP_DUP", "DUP":
			result = append(result, txscript.OP_DUP)
		case "OP_HASH160", "HASH160":
			result = append(result, txscript.OP_HASH160)
		case "OP_EQUALVERIFY", "EQUALVERIFY":
			result = append(result, txscript.OP_EQUALVERIFY)
		case "OP_CHECKSIG", "CHECKSIG":
			result = append(result, txscript.OP_CHECKSIG)
		case "OP_CHECKMULTISIG", "CHECKMULTISIG":
			result = append(result, txscript.OP_CHECKMULTISIG)
		case "OP_EQUAL", "EQUAL":
			result = append(result, txscript.OP_EQUAL)
		default:
			// Try to parse as a number
			if n, err := strconv.ParseInt(token, 10, 64); err == nil {
				if n >= -1 && n <= 16 {
					if n == -1 {
						result = append(result, txscript.OP_1NEGATE)
					} else if n == 0 {
						result = append(result, txscript.OP_0)
					} else {
						result = append(result, byte(txscript.OP_1+n-1))
					}
					continue
				}
			}
			return nil, fmt.Errorf("unknown token: %s", token)
		}
	}

	return result, nil
}

func scriptToShortForm(script []byte) string {
	var parts []string
	for i := 0; i < len(script); {
		op := script[i]
		i++

		// Data push opcodes
		if op >= 0x01 && op <= 0x4b {
			pushLen := int(op)
			if i+pushLen > len(script) {
				break
			}
			parts = append(parts, fmt.Sprintf("0x%02x", op))
			parts = append(parts, "0x"+hex.EncodeToString(script[i:i+pushLen]))
			i += pushLen
			continue
		}

		// Named opcodes
		switch op {
		case txscript.OP_0:
			parts = append(parts, "0")
		case txscript.OP_DUP:
			parts = append(parts, "DUP")
		case txscript.OP_HASH160:
			parts = append(parts, "HASH160")
		case txscript.OP_EQUALVERIFY:
			parts = append(parts, "EQUALVERIFY")
		case txscript.OP_CHECKSIG:
			parts = append(parts, "CHECKSIG")
		case txscript.OP_CHECKMULTISIG:
			parts = append(parts, "CHECKMULTISIG")
		case txscript.OP_EQUAL:
			parts = append(parts, "EQUAL")
		default:
			parts = append(parts, fmt.Sprintf("0x%02x", op))
		}
	}

	return strings.Join(parts, " ")
}

func formatScriptTestsJSON(data []byte) []byte {
	// Keep the default marshaling for now
	// The original file has specific formatting but functionality is the same
	return data
}
