// Copyright (c) 2015-2016 The btcsuite developers
// Copyright (c) 2016-2021 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package blockchain

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/wire"
)

// -----------------------------------------------------------------------------
// The unspent transaction output (utxo) set consists of an entry for each
// unspent output using a format that is optimized to reduce space using domain
// specific compression algorithms.
//
// Each entry is keyed by an outpoint as specified below.  It is important to
// note that the key encoding uses a VLQ, which employs an MSB encoding so
// iteration of utxos when doing byte-wise comparisons will produce them in
// order.
//
// The serialized key format is:
//
//   <hash><tree><output index>
//
//   Field                Type             Size
//   hash                 chainhash.Hash   chainhash.HashSize
//   tree                 VLQ              variable
//   output index         VLQ              variable
//
// The serialized value format is:
//
//   <block height><block index><flags><coin type><compressed txout>
//   OPTIONAL: [<ticket min outs>]
//
//   Field                Type     Size
//   block height         VLQ      variable
//   block index          VLQ      variable
//   flags                VLQ      variable
//   coin type            VLQ      variable (new for dual-coin support)
//   compressed txout (format depends on coin type):
//     VAR (coinType == 0):
//       compressed amount   VLQ      variable (standard compression)
//       script version      VLQ      variable
//       compressed script   []byte   variable
//     SKA (coinType > 0):
//       amount length       uint8    1 byte (length of big.Int bytes)
//       amount bytes        []byte   variable (big-endian big.Int)
//       script version      VLQ      variable
//       compressed script   []byte   variable
//
//   OPTIONAL
//     ticketMinOuts      []byte         variable
//
// The serialized flags format is:
//   bit  0     - containing transaction is a coinbase
//   bit  1     - containing transaction has an expiry
//   bits 2-5   - transaction type
//   bits 6-7   - unused
//
// The ticket min outs field contains minimally encoded outputs for all outputs
// of a ticket transaction. It is only encoded for ticket submission outputs.
//
// -----------------------------------------------------------------------------

// maxUint32VLQSerializeSize is the maximum number of bytes a max uint32 takes
// to serialize as a VLQ.
var maxUint32VLQSerializeSize = serializeSizeVLQ(1<<32 - 1)

// maxUint8VLQSerializeSize is the maximum number of bytes a max uint8 takes to
// serialize as a VLQ.
var maxUint8VLQSerializeSize = serializeSizeVLQ(1<<8 - 1)

// utxoSetDbPrefixSize is the number of bytes that the prefix for UTXO set
// entries takes.
var utxoSetDbPrefixSize = len(utxoPrefixUtxoSet)

// outpointKeyPool defines a concurrent safe free list of byte slices used to
// provide temporary buffers for outpoint database keys.
var outpointKeyPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, utxoSetDbPrefixSize+chainhash.HashSize+
			maxUint8VLQSerializeSize+maxUint32VLQSerializeSize)
		return &b // Pointer to slice to avoid boxing alloc.
	},
}

// outpointKey returns a key suitable for use as a database key in the utxo set
// while making use of a free list.  A new buffer is allocated if there are not
// already any available on the free list.  The returned byte slice should be
// returned to the free list by using the recycleOutpointKey function when the
// caller is done with it _unless_ the slice will need to live for longer than
// the caller can calculate such as when used to write to the database.
func outpointKey(outpoint wire.OutPoint) *[]byte {
	// A VLQ employs an MSB encoding, so they are useful not only to reduce the
	// amount of storage space, but also so iteration of utxos when doing
	// byte-wise comparisons will produce them in order.
	key := outpointKeyPool.Get().(*[]byte)
	tree := uint64(outpoint.Tree)
	idx := uint64(outpoint.Index)
	*key = (*key)[:utxoSetDbPrefixSize+chainhash.HashSize+
		serializeSizeVLQ(tree)+serializeSizeVLQ(idx)]
	copy(*key, utxoPrefixUtxoSet)
	offset := utxoSetDbPrefixSize
	copy((*key)[offset:], outpoint.Hash[:])
	offset += chainhash.HashSize
	offset += putVLQ((*key)[offset:], tree)
	putVLQ((*key)[offset:], idx)
	return key
}

// decodeOutpointKey decodes the passed serialized key into the passed outpoint.
func decodeOutpointKey(serialized []byte, outpoint *wire.OutPoint) error {
	if utxoSetDbPrefixSize+chainhash.HashSize >= len(serialized) {
		return errDeserialize("unexpected length for serialized outpoint key")
	}

	// Ignore the UTXO set prefix.
	offset := utxoSetDbPrefixSize

	// Deserialize the hash.
	var hash chainhash.Hash
	copy(hash[:], serialized[offset:offset+chainhash.HashSize])
	offset += chainhash.HashSize

	// Deserialize the tree.
	tree, bytesRead := deserializeVLQ(serialized[offset:])
	offset += bytesRead
	if offset >= len(serialized) {
		return errDeserialize("unexpected end of data after tree")
	}

	// Deserialize the index.
	idx, _ := deserializeVLQ(serialized[offset:])

	// Populate the outpoint.
	outpoint.Hash = hash
	outpoint.Tree = int8(tree)
	outpoint.Index = uint32(idx)

	return nil
}

// recycleOutpointKey puts the provided byte slice, which should have been
// obtained via the outpointKey function, back on the free list.
func recycleOutpointKey(key *[]byte) {
	outpointKeyPool.Put(key)
}

// serializeUtxoEntry returns the entry serialized to a format that is suitable
// for long-term storage.  The format is described in detail above.
func serializeUtxoEntry(entry *UtxoEntry) []byte {
	// Spent entries have no serialization.
	if entry.IsSpent() {
		return nil
	}

	// Calculate the size needed to serialize the entry.
	flags := encodeFlags(entry.IsCoinBase(), entry.HasExpiry(),
		entry.TransactionType())

	// Base size (block height, block index, flags, coin type)
	size := serializeSizeVLQ(uint64(entry.blockHeight)) +
		serializeSizeVLQ(uint64(entry.blockIndex)) +
		serializeSizeVLQ(uint64(flags)) +
		serializeSizeVLQ(uint64(entry.coinType))

	// Amount size depends on coin type
	var skaAmountBytes []byte
	if entry.coinType.IsSKA() {
		// SKA: 1-byte length prefix + big.Int bytes
		if entry.skaAmount != nil {
			skaAmountBytes = entry.skaAmount.Bytes()
		}
		size += 1 + len(skaAmountBytes) // length byte + amount bytes
		// Script (no compressed amount for SKA)
		size += serializeSizeVLQ(uint64(entry.scriptVersion)) +
			compressedScriptSize(entry.scriptVersion, entry.pkScript)
	} else {
		// VAR: Use standard compressed txout
		const hasAmount = true
		size += compressedTxOutSize(uint64(entry.amount), entry.scriptVersion,
			entry.pkScript, hasAmount)
	}

	if entry.ticketMinOuts != nil {
		size += len(entry.ticketMinOuts.data)
	}

	// Serialize the entry.
	serialized := make([]byte, size)
	offset := putVLQ(serialized, uint64(entry.blockHeight))
	offset += putVLQ(serialized[offset:], uint64(entry.blockIndex))
	offset += putVLQ(serialized[offset:], uint64(flags))
	offset += putVLQ(serialized[offset:], uint64(entry.coinType))

	// Serialize amount based on coin type
	if entry.coinType.IsSKA() {
		// SKA: Write length byte + big.Int bytes (big-endian)
		serialized[offset] = byte(len(skaAmountBytes))
		offset++
		copy(serialized[offset:], skaAmountBytes)
		offset += len(skaAmountBytes)
		// Write script version and compressed script
		offset += putVLQ(serialized[offset:], uint64(entry.scriptVersion))
		offset += putCompressedScript(serialized[offset:], entry.scriptVersion, entry.pkScript)
	} else {
		// VAR: Standard compressed txout
		const hasAmount = true
		offset += putCompressedTxOut(serialized[offset:], uint64(entry.amount),
			entry.scriptVersion, entry.pkScript, hasAmount)
	}

	if entry.ticketMinOuts != nil {
		copy(serialized[offset:], entry.ticketMinOuts.data)
	}

	return serialized
}

// deserializeUtxoEntry decodes a utxo entry from the passed serialized byte
// slice into a new UtxoEntry using a format that is suitable for long-term
// storage.  The format is described in detail above.
//
// This function automatically detects whether the entry is in version 3 format
// (without coin type) or version 4+ format (with coin type) and handles both.
// SKA entries use big.Int for amounts, while VAR entries use int64.
func deserializeUtxoEntry(serialized []byte, txOutIndex uint32) (*UtxoEntry, error) {
	// Deserialize the block height.
	blockHeight, bytesRead := deserializeVLQ(serialized)
	offset := bytesRead
	if offset >= len(serialized) {
		return nil, errDeserialize("unexpected end of data after height")
	}

	// Deserialize the block index.
	blockIndex, bytesRead := deserializeVLQ(serialized[offset:])
	offset += bytesRead
	if offset >= len(serialized) {
		return nil, errDeserialize("unexpected end of data after index")
	}

	// Deserialize the flags.
	flags, bytesRead := deserializeVLQ(serialized[offset:])
	offset += bytesRead
	if offset >= len(serialized) {
		return nil, errDeserialize("unexpected end of data after flags")
	}
	isCoinBase, hasExpiry, txType := decodeFlags(txOutFlags(flags))

	// Try to deserialize coin type for version 4+ format.
	// V3 format: [height][index][flags][compressed txout] - no coin type
	// V4 VAR:    [height][index][flags][coin type=0][compressed txout]
	// V4 SKA:    [height][index][flags][coin type>0][ska amount len][ska amount][script version][script]
	//
	// Detection strategy:
	// 1. Read potential coin type value
	// 2. If value == 0: try V4 VAR format, fall back to V3 if it fails
	// 3. If value > 0: try V4 SKA format, fall back to V3 if it fails
	// The key insight is that V3 never existed with SKA, so any SKA data MUST be V4.
	// For VAR, we need to try both because the first VLQ after flags in V3 could be 0.
	var coinType cointype.CoinType = cointype.CoinTypeVAR // Default for legacy entries

	coinTypeVal, bytesRead := deserializeVLQ(serialized[offset:])
	nextOffset := offset + bytesRead

	// Determine if this is V4 format
	isV4Format := false
	if nextOffset < len(serialized) {
		potentialCoinType := cointype.CoinType(coinTypeVal)
		if potentialCoinType.IsSKA() {
			// Potential SKA coin type (1-255) - try V4 SKA format
			// SKA format has: [ska amount len:1 byte][ska amount bytes][script version][compressed script]
			// Validate by trying to fully decode as SKA format
			if nextOffset < len(serialized) {
				skaAmountLen := int(serialized[nextOffset])
				// Sanity check: amount length should be reasonable (0-32 bytes for big.Int)
				if skaAmountLen <= 32 {
					skaOffset := nextOffset + 1 + skaAmountLen
					// Try to decode the rest as script version + compressed script
					if skaOffset < len(serialized) {
						// Read script version
						_, svBytesRead := deserializeVLQ(serialized[skaOffset:])
						if svBytesRead > 0 && skaOffset+svBytesRead <= len(serialized) {
							// Try to decode compressed script
							scriptOffset := skaOffset + svBytesRead
							scriptSize := decodeCompressedScriptSize(serialized[scriptOffset:])
							if scriptSize >= 0 && scriptOffset+scriptSize <= len(serialized) {
								// Successfully validated as V4 SKA format
								coinType = potentialCoinType
								offset = nextOffset
								isV4Format = true
							}
						}
					}
				}
			}
			// If SKA validation failed, fall back to V3 VAR (keep offset as is)
		} else if coinTypeVal == 0 {
			// Could be V4 VAR (coinType=0) or V3 format
			// Try to decode as compressed txout to determine
			_, _, _, _, err := decodeCompressedTxOut(serialized[nextOffset:], true)
			if err == nil {
				// Successfully decoded with coin type 0, this is V4 VAR format
				coinType = cointype.CoinTypeVAR
				offset = nextOffset
				isV4Format = true
			}
			// If decoding failed, this is V3 format, keep offset as is
		}
	}

	// Decode amount and script based on coin type
	var amount int64
	var skaAmount *big.Int
	var scriptVersion uint16
	var script []byte

	if isV4Format && coinType.IsSKA() {
		// SKA format: [length:1 byte][amount bytes:N bytes][script version][compressed script]
		if offset >= len(serialized) {
			return nil, errDeserialize("unexpected end of data before SKA amount length")
		}
		amountLen := int(serialized[offset])
		offset++

		if offset+amountLen > len(serialized) {
			return nil, errDeserialize("unexpected end of data during SKA amount")
		}
		if amountLen > 0 {
			skaAmount = new(big.Int).SetBytes(serialized[offset : offset+amountLen])
		} else {
			skaAmount = big.NewInt(0)
		}
		offset += amountLen

		// Decode script version
		var scriptVersionVal uint64
		scriptVersionVal, bytesRead = deserializeVLQ(serialized[offset:])
		if bytesRead == 0 {
			return nil, errDeserialize("unexpected end of data during script version")
		}
		scriptVersion = uint16(scriptVersionVal)
		offset += bytesRead

		// Decode compressed script
		scriptSize := decodeCompressedScriptSize(serialized[offset:])
		if scriptSize < 0 {
			return nil, errDeserialize("negative script size")
		}
		if offset+scriptSize > len(serialized) {
			return nil, errDeserialize(fmt.Sprintf("unexpected end of data after script size (got %v, need %v)",
				len(serialized)-offset, scriptSize))
		}
		script = decompressScript(serialized[offset : offset+scriptSize])
		offset += scriptSize
	} else {
		// VAR format: Standard compressed txout
		var err error
		amount, scriptVersion, script, bytesRead, err =
			decodeCompressedTxOut(serialized[offset:], true)
		if err != nil {
			return nil, errDeserialize(fmt.Sprintf("unable to decode utxo: %v", err))
		}
		offset += bytesRead
	}

	// Create a new utxo entry with the details deserialized above.
	entry := &UtxoEntry{
		amount:        amount,
		skaAmount:     skaAmount,
		pkScript:      script,
		blockHeight:   uint32(blockHeight),
		blockIndex:    uint32(blockIndex),
		scriptVersion: scriptVersion,
		coinType:      coinType,
		packedFlags:   encodeUtxoFlags(isCoinBase, hasExpiry, txType),
	}

	// Copy the minimal outputs if this was a ticket submission output.
	if isTicketSubmissionOutput(txType, txOutIndex) {
		sz, err := readDeserializeSizeOfMinimalOutputs(serialized[offset:])
		if err != nil {
			return nil, errDeserialize(fmt.Sprintf("unable to decode "+
				"ticket outputs: %v", err))
		}
		entry.ticketMinOuts = &ticketMinimalOutputs{
			data: make([]byte, sz),
		}
		copy(entry.ticketMinOuts.data, serialized[offset:offset+sz])
	}

	return entry, nil
}

// -----------------------------------------------------------------------------
// The utxo set state contains information regarding the current state of the
// utxo set.  In particular, it tracks the block height and block hash of the
// last completed flush.
//
// The utxo set state is tracked in the database since at any given time, the
// utxo cache may not be consistent with the utxo set in the database.  This is
// due to the fact that the utxo cache only flushes changes to the database
// periodically.  Therefore, during initialization, the utxo set state is used
// to identify the last flushed state of the utxo set and it can be caught up
// to the current best state of the main chain.
//
// Note: The utxo set state MUST always be updated in the same database
// transaction that the utxo set is updated in to guarantee that they stay in
// sync in the database.
//
// The serialized format is:
//
//   <block height><block hash>
//
//   Field          Type             Size
//   block height   VLQ              variable
//   block hash     chainhash.Hash   chainhash.HashSize
//
// -----------------------------------------------------------------------------

// UtxoSetState represents the current state of the utxo set.  In particular,
// it tracks the block height and block hash of the last completed flush.
type UtxoSetState struct {
	lastFlushHeight uint32
	lastFlushHash   chainhash.Hash
}

// serializeUtxoSetState serializes the provided utxo set state.  The format is
// described in detail above.
func serializeUtxoSetState(state *UtxoSetState) []byte {
	// Calculate the size needed to serialize the utxo set state.
	size := serializeSizeVLQ(uint64(state.lastFlushHeight)) + chainhash.HashSize

	// Serialize the utxo set state and return it.
	serialized := make([]byte, size)
	offset := putVLQ(serialized, uint64(state.lastFlushHeight))
	copy(serialized[offset:], state.lastFlushHash[:])
	return serialized
}

// deserializeUtxoSetState deserializes the passed serialized byte slice into
// the utxo set state.  The format is described in detail above.
func deserializeUtxoSetState(serialized []byte) (*UtxoSetState, error) {
	// Deserialize the block height.
	blockHeight, bytesRead := deserializeVLQ(serialized)
	offset := bytesRead
	if offset >= len(serialized) {
		return nil, errDeserialize("unexpected end of data after height")
	}

	// Deserialize the hash.
	if len(serialized[offset:]) != chainhash.HashSize {
		return nil, errDeserialize("unexpected length for serialized hash")
	}
	var hash chainhash.Hash
	copy(hash[:], serialized[offset:offset+chainhash.HashSize])

	// Create the utxo set state and return it.
	return &UtxoSetState{
		lastFlushHeight: uint32(blockHeight),
		lastFlushHash:   hash,
	}, nil
}
