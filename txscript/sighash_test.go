// Copyright (c) 2013-2015 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

// TestVarIntSerializeSize ensures the serialize size for variable length
// integers works as intended.
func TestVarIntSerializeSize(t *testing.T) {
	tests := []struct {
		val  uint64 // Value to get the serialized size for
		size int    // Expected serialized size
	}{

		{0, 1},                  // Single byte encoded
		{0xfc, 1},               // Max single byte encoded
		{0xfd, 3},               // Min 3-byte encoded
		{0xffff, 3},             // Max 3-byte encoded
		{0x10000, 5},            // Min 5-byte encoded
		{0xffffffff, 5},         // Max 5-byte encoded
		{0x100000000, 9},        // Min 9-byte encoded
		{0xffffffffffffffff, 9}, // Max 9-byte encoded
	}

	for i, test := range tests {
		serializedSize := varIntSerializeSize(test.val)
		if serializedSize != test.size {
			t.Errorf("varIntSerializeSize #%d got: %d, want: %d", i,
				serializedSize, test.size)
			continue
		}
	}
}

// TestPutVarInt ensures encoding variable length integers works as intended.
func TestPutVarInt(t *testing.T) {
	tests := []struct {
		val     uint64 // Value to encode
		encoded []byte // expected encoding
	}{

		{0, hexToBytes("00")},                                  // Single byte
		{0xfc, hexToBytes("fc")},                               // Max single
		{0xfd, hexToBytes("fdfd00")},                           // Min 3-byte
		{0xffff, hexToBytes("fdffff")},                         // Max 3-byte
		{0x10000, hexToBytes("fe00000100")},                    // Min 5-byte
		{0xffffffff, hexToBytes("feffffffff")},                 // Max 5-byte
		{0x100000000, hexToBytes("ff0000000001000000")},        // Min 9-byte
		{0xffffffffffffffff, hexToBytes("ffffffffffffffffff")}, // Max 9-byte
	}

	for i, test := range tests {
		encoded := make([]byte, varIntSerializeSize(test.val))
		gotBytesWritten := putVarInt(encoded, test.val)
		if !bytes.Equal(encoded, test.encoded) {
			t.Errorf("putVarInt #%d\n got: %x want: %x", i, encoded,
				test.encoded)
			continue
		}
		if gotBytesWritten != len(test.encoded) {
			t.Errorf("putVarInt: did not get expected number of bytes written "+
				"for %d - got %d, want %d", test.val, gotBytesWritten,
				len(test.encoded))
			continue
		}
	}
}

// TestCalcSignatureHash does some rudimentary testing of msg hash calculation.
func TestCalcSignatureHash(t *testing.T) {
	tx := new(wire.MsgTx)
	tx.SerType = wire.TxSerializeFull
	tx.Version = 1
	for i := 0; i < 3; i++ {
		txIn := new(wire.TxIn)
		txIn.Sequence = 0xFFFFFFFF
		txIn.PreviousOutPoint.Hash = chainhash.HashH([]byte{byte(i)})
		txIn.PreviousOutPoint.Index = uint32(i)
		txIn.PreviousOutPoint.Tree = int8(0)
		tx.AddTxIn(txIn)
	}
	for i := 0; i < 2; i++ {
		txOut := new(wire.TxOut)
		txOut.PkScript = hexToBytes("51")
		txOut.Value = 0x0000FF00FF00FF00
		tx.AddTxOut(txOut)
	}

	// Digest reflects the Monetarium sighash extension that commits to
	// per-input SKAValueIn and per-output CoinType + SKAValue.  The test
	// transaction has all-VAR inputs and outputs, so each input contributes
	// a single 0x00 SKA-length byte and each output contributes a 0x00
	// CoinType byte plus a 0x00 SKA-length byte.
	want := hexToBytes("c7cf006916b72cc99fecd59e9cde4bbc09d1cfe9ff496610" +
		"b63ca2106c89d74f")
	script := hexToBytes("51")

	// Test prefix caching.
	msg1, err := CalcSignatureHash(script, SigHashAll, tx, 0, nil)
	if err != nil {
		t.Fatalf("unexpected error %v", err.Error())
	}

	prefixHash := tx.TxHash()
	msg2, err := CalcSignatureHash(script, SigHashAll, tx, 0, &prefixHash)
	if err != nil {
		t.Fatalf("unexpected error %v", err.Error())
	}

	if !bytes.Equal(msg1, want) {
		t.Errorf("for sighash all sig noncached wrong msg -- got %x, want %x",
			msg1,
			want)
	}
	if !bytes.Equal(msg2, want) {
		t.Errorf("for sighash all sig cached wrong msg -- got %x, want %x",
			msg1,
			want)
	}
	if !bytes.Equal(msg1, msg2) {
		t.Errorf("for sighash all sig non-equivalent msgs %x and %x were "+
			"returned when using a cached prefix",
			msg1,
			msg2)
	}

	// Move the index and make sure that we get a whole new hash, despite
	// using the same TxOuts.
	msg3, err := CalcSignatureHash(script, SigHashAll, tx, 1, &prefixHash)
	if err != nil {
		t.Fatalf("unexpected error %v", err.Error())
	}

	if bytes.Equal(msg1, msg3) {
		t.Errorf("for sighash all sig equivalent msgs %x and %x were "+
			"returned when using a cached prefix but different indices",
			msg1, msg3)
	}
}

// buildSKATestTx constructs a transaction with one SKA input and one SKA
// output for use in the sighash-binding tests below.  The exact field values
// are not consensus-meaningful here; only the SKA atom amounts are.
func buildSKATestTx(skaIn, skaOut *big.Int) *wire.MsgTx {
	tx := new(wire.MsgTx)
	tx.SerType = wire.TxSerializeFull
	tx.Version = 1
	txIn := new(wire.TxIn)
	txIn.Sequence = 0xFFFFFFFF
	txIn.PreviousOutPoint.Hash = chainhash.HashH([]byte{0x42})
	txIn.PreviousOutPoint.Index = 0
	txIn.PreviousOutPoint.Tree = 0
	txIn.SKAValueIn = new(big.Int).Set(skaIn)
	tx.AddTxIn(txIn)
	txOut := new(wire.TxOut)
	txOut.PkScript = hexToBytes("51")
	txOut.CoinType = cointype.CoinType(1)
	txOut.SKAValue = new(big.Int).Set(skaOut)
	tx.AddTxOut(txOut)
	return tx
}

// TestCalcSignatureHashSKAOutput verifies the sighash digest changes when an
// SKA output amount is tampered with — the property that protects offline
// signers from a host that lies about the SKA amount being authorized.
func TestCalcSignatureHashSKAOutput(t *testing.T) {
	script := hexToBytes("51")
	original := buildSKATestTx(big.NewInt(1_000_000), big.NewInt(900_000))
	tampered := buildSKATestTx(big.NewInt(1_000_000), big.NewInt(900_001))

	digestOriginal, err := CalcSignatureHash(script, SigHashAll, original, 0, nil)
	if err != nil {
		t.Fatalf("digest original: %v", err)
	}
	digestTampered, err := CalcSignatureHash(script, SigHashAll, tampered, 0, nil)
	if err != nil {
		t.Fatalf("digest tampered: %v", err)
	}
	if bytes.Equal(digestOriginal, digestTampered) {
		t.Fatalf("sighash did not change when SKAValue was tampered: %x", digestOriginal)
	}
}

// TestCalcSignatureHashSKAInput verifies the sighash digest changes when an
// SKA input amount is tampered with — symmetric coverage to the output case.
func TestCalcSignatureHashSKAInput(t *testing.T) {
	script := hexToBytes("51")
	original := buildSKATestTx(big.NewInt(1_000_000), big.NewInt(900_000))
	tampered := buildSKATestTx(big.NewInt(2_000_000), big.NewInt(900_000))

	digestOriginal, err := CalcSignatureHash(script, SigHashAll, original, 0, nil)
	if err != nil {
		t.Fatalf("digest original: %v", err)
	}
	digestTampered, err := CalcSignatureHash(script, SigHashAll, tampered, 0, nil)
	if err != nil {
		t.Fatalf("digest tampered: %v", err)
	}
	if bytes.Equal(digestOriginal, digestTampered) {
		t.Fatalf("sighash did not change when SKAValueIn was tampered: %x", digestOriginal)
	}
}

// TestCalcSignatureHashCoinTypeBinding verifies the sighash digest changes
// when only the per-output CoinType byte is altered.  This prevents an
// attacker from re-tagging a VAR output as SKA (or swapping SKA-1 for SKA-2)
// without invalidating any prior signature on the transaction.
func TestCalcSignatureHashCoinTypeBinding(t *testing.T) {
	script := hexToBytes("51")
	original := buildSKATestTx(big.NewInt(1_000_000), big.NewInt(900_000))
	tampered := buildSKATestTx(big.NewInt(1_000_000), big.NewInt(900_000))
	tampered.TxOut[0].CoinType = cointype.CoinType(2)

	digestOriginal, err := CalcSignatureHash(script, SigHashAll, original, 0, nil)
	if err != nil {
		t.Fatalf("digest original: %v", err)
	}
	digestTampered, err := CalcSignatureHash(script, SigHashAll, tampered, 0, nil)
	if err != nil {
		t.Fatalf("digest tampered: %v", err)
	}
	if bytes.Equal(digestOriginal, digestTampered) {
		t.Fatalf("sighash did not change when CoinType was tampered: %x", digestOriginal)
	}
}

// TestCalcSignatureHashSKADeterministic locks in the on-wire encoding so a
// future refactor cannot silently change the sighash format.  If the digest
// here drifts, every previously-signed SKA transaction would need to be
// resigned — that is a consensus break and demands an explicit decision.
func TestCalcSignatureHashSKADeterministic(t *testing.T) {
	script := hexToBytes("51")
	tx := buildSKATestTx(big.NewInt(1_000_000), big.NewInt(900_000))

	want := "c6d345434cc91115f9171841acc2e908dd1cdb89cceef5b06aa58228da8dc47d"
	got, err := CalcSignatureHash(script, SigHashAll, tx, 0, nil)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if hex := bytesToHex(got); hex != want {
		t.Fatalf("digest drift: got %s, want %s", hex, want)
	}
}

func bytesToHex(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2] = hexdigits[c>>4]
		out[i*2+1] = hexdigits[c&0xf]
	}
	return string(out)
}
