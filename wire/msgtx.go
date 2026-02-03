// Copyright (c) 2013-2016 The btcsuite developers
// Copyright (c) 2015-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"fmt"
	"io"
	"math/big"
	"strconv"

	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/cointype"
)

// FeesByType represents transaction fees collected by coin type.
// All fees use *big.Int for consistency - VAR values fit in int64
// and can be safely converted with .Int64() when needed.
type FeesByType map[cointype.CoinType]*big.Int

// NewFeesByType creates a new empty fees-by-type map.
func NewFeesByType() FeesByType {
	return make(FeesByType)
}

// Add adds a fee amount (as int64) to the specified coin type.
// For VAR, this is the natural representation. For SKA with large fees,
// use AddBig instead to preserve full precision.
func (f FeesByType) Add(coinType cointype.CoinType, amount int64) {
	f.AddBig(coinType, big.NewInt(amount))
}

// AddBig adds a bigint fee amount to the specified coin type.
func (f FeesByType) AddBig(coinType cointype.CoinType, amount *big.Int) {
	if amount == nil || amount.Sign() == 0 {
		return
	}
	existing := f[coinType]
	if existing == nil {
		f[coinType] = new(big.Int).Set(amount)
	} else {
		f[coinType] = new(big.Int).Add(existing, amount)
	}
}

// Get returns the total fees for the specified coin type as int64.
// Returns 0 if no fees exist or if the value exceeds int64.
// For VAR, this is always safe. For SKA with large fees, use GetBig.
func (f FeesByType) Get(coinType cointype.CoinType) int64 {
	if fee := f[coinType]; fee != nil && fee.IsInt64() {
		return fee.Int64()
	}
	return 0
}

// GetBig returns the total fees for the specified coin type as *big.Int.
// Returns nil if no fees exist for this coin type.
func (f FeesByType) GetBig(coinType cointype.CoinType) *big.Int {
	return f[coinType]
}

// AddSKA is an alias for AddBig for SKA fee operations.
func (f FeesByType) AddSKA(coinType cointype.CoinType, amount *big.Int) {
	f.AddBig(coinType, amount)
}

// GetSKA is an alias for GetBig for SKA fee operations.
func (f FeesByType) GetSKA(coinType cointype.CoinType) *big.Int {
	return f.GetBig(coinType)
}

// Types returns a slice of all coin types that have non-zero fees.
func (f FeesByType) Types() []cointype.CoinType {
	types := make([]cointype.CoinType, 0, len(f))
	for coinType, amount := range f {
		if amount != nil && amount.Sign() > 0 {
			types = append(types, coinType)
		}
	}
	return types
}

// Merge adds all fees from another FeesByType into this one.
func (f FeesByType) Merge(other FeesByType) {
	for coinType, amount := range other {
		if amount != nil && amount.Sign() > 0 {
			f.AddBig(coinType, amount)
		}
	}
}

// HasFee returns true if there is a non-zero fee for the given coin type.
func (f FeesByType) HasFee(coinType cointype.CoinType) bool {
	fee := f[coinType]
	return fee != nil && fee.Sign() > 0
}

// HasSKAFees returns true if there are any non-zero fees for SKA coin types.
func (f FeesByType) HasSKAFees() bool {
	for coinType, amount := range f {
		if coinType.IsSKA() && amount != nil && amount.Sign() > 0 {
			return true
		}
	}
	return false
}

// SKATypes returns a slice of all SKA coin types that have non-zero fees.
func (f FeesByType) SKATypes() []cointype.CoinType {
	types := make([]cointype.CoinType, 0)
	for coinType, amount := range f {
		if coinType.IsSKA() && amount != nil && amount.Sign() > 0 {
			types = append(types, coinType)
		}
	}
	return types
}

// GetPrimaryCoinType determines the primary coin type of a transaction by
// examining its outputs. Returns the first non-zero coin type found, or
// CoinTypeVAR if all outputs are VAR or no outputs exist.
func GetPrimaryCoinType(tx *MsgTx) cointype.CoinType {
	for _, txOut := range tx.TxOut {
		if txOut.CoinType != cointype.CoinTypeVAR {
			return txOut.CoinType
		}
	}
	return cointype.CoinTypeVAR
}

// CalcTxFee calculates the transaction fee as *big.Int, handling both
// VAR and SKA coin types correctly. For VAR transactions, it uses ValueIn/Value.
// For SKA transactions, it uses SKAValueIn/SKAValue.
func CalcTxFee(tx *MsgTx) (fee *big.Int, coinType cointype.CoinType) {
	coinType = GetPrimaryCoinType(tx)

	if coinType.IsSKA() {
		totalIn := new(big.Int)
		for _, txIn := range tx.TxIn {
			if txIn.SKAValueIn != nil {
				totalIn.Add(totalIn, txIn.SKAValueIn)
			}
		}

		totalOut := new(big.Int)
		for _, txOut := range tx.TxOut {
			if txOut.SKAValue != nil {
				totalOut.Add(totalOut, txOut.SKAValue)
			}
		}

		return new(big.Int).Sub(totalIn, totalOut), coinType
	}

	// VAR transaction
	var totalIn, totalOut int64
	for _, txIn := range tx.TxIn {
		totalIn += txIn.ValueIn
	}
	for _, txOut := range tx.TxOut {
		totalOut += txOut.Value
	}

	return big.NewInt(totalIn - totalOut), coinType
}

// CalcFeeSplitByCoinType applies fee split to multi-coin fees based on
// work/stake proportions. Returns separate FeesByType for miners and stakers.
// The proportions should be obtained from the subsidy system based on the
// current subsidy split variant (e.g., SSVMonetarium).
// All fees use bigint arithmetic for consistency.
func CalcFeeSplitByCoinType(feesByType FeesByType, workProportion, stakeProportion uint16) (minerFees, stakerFees FeesByType) {
	minerFees = NewFeesByType()
	stakerFees = NewFeesByType()

	if feesByType == nil {
		return minerFees, stakerFees
	}

	// Fees are split only between miners and stakers (treasury gets no fees)
	adjustedTotal := int64(workProportion + stakeProportion)
	if adjustedTotal == 0 {
		// Shouldn't happen with valid proportions, but handle gracefully
		return minerFees, stakerFees
	}

	adjustedTotalBig := big.NewInt(adjustedTotal)
	workProportionBig := big.NewInt(int64(workProportion))
	stakeProportionBig := big.NewInt(int64(stakeProportion))

	for coinType, totalFee := range feesByType {
		if totalFee == nil || totalFee.Sign() <= 0 {
			continue
		}

		// Calculate proportional split: (totalFee * proportion) / adjustedTotal
		minerFee := new(big.Int).Mul(totalFee, workProportionBig)
		minerFee.Div(minerFee, adjustedTotalBig)

		stakerFee := new(big.Int).Mul(totalFee, stakeProportionBig)
		stakerFee.Div(stakerFee, adjustedTotalBig)

		// Handle any rounding remainder by giving it to miners
		distributed := new(big.Int).Add(minerFee, stakerFee)
		remainder := new(big.Int).Sub(totalFee, distributed)
		minerFee.Add(minerFee, remainder)

		if minerFee.Sign() > 0 {
			minerFees[coinType] = minerFee
		}
		if stakerFee.Sign() > 0 {
			stakerFees[coinType] = stakerFee
		}
	}

	return minerFees, stakerFees
}

const (
	// TxVersion is the initial transaction version.
	TxVersion uint16 = 1

	// TxVersionSeqLock is the transaction version that enables sequence
	// locks.
	TxVersionSeqLock uint16 = 2

	// TxVersionTreasury is the transaction version that enables the
	// decentralized treasury features.
	TxVersionTreasury uint16 = 3

	// MaxTxInSequenceNum is the maximum sequence number the sequence field
	// of a transaction input can be.
	MaxTxInSequenceNum uint32 = 0xffffffff

	// MaxPrevOutIndex is the maximum index the index field of a previous
	// outpoint can be.
	MaxPrevOutIndex uint32 = 0xffffffff

	// NoExpiryValue is the value of expiry that indicates the transaction
	// has no expiry.
	NoExpiryValue uint32 = 0

	// NullValueIn is a null value for an input witness.
	NullValueIn int64 = -1

	// NullBlockHeight is the null value for an input witness. It references
	// the genesis block.
	NullBlockHeight uint32 = 0x00000000

	// NullBlockIndex is the null transaction index in a block for an input
	// witness.
	NullBlockIndex uint32 = 0xffffffff

	// DefaultPkScriptVersion is the default pkScript version, referring to
	// extended Decred script.
	DefaultPkScriptVersion uint16 = 0x0000

	// TxTreeUnknown is the value returned for a transaction tree that is
	// unknown.  This is typically because the transaction has not been
	// inserted into a block yet.
	TxTreeUnknown int8 = -1

	// TxTreeRegular is the value for a normal transaction tree for a
	// transaction's location in a block.
	TxTreeRegular int8 = 0

	// TxTreeStake is the value for a stake transaction tree for a
	// transaction's location in a block.
	TxTreeStake int8 = 1

	// SequenceLockTimeDisabled is a flag that if set on a transaction
	// input's sequence number, the sequence number will not be interpreted
	// as a relative locktime.
	SequenceLockTimeDisabled = 1 << 31

	// SequenceLockTimeIsSeconds is a flag that if set on a transaction
	// input's sequence number, the relative locktime has units of 512
	// seconds.
	SequenceLockTimeIsSeconds = 1 << 22

	// SequenceLockTimeMask is a mask that extracts the relative locktime
	// when masked against the transaction input sequence number.
	SequenceLockTimeMask = 0x0000ffff

	// SequenceLockTimeGranularity is the defined time based granularity
	// for seconds-based relative time locks.  When converting from seconds
	// to a sequence number, the value is right shifted by this amount,
	// therefore the granularity of relative time locks in 512 or 2^9
	// seconds.  Enforced relative lock times are multiples of 512 seconds.
	SequenceLockTimeGranularity = 9
)

const (
	// defaultTxInOutAlloc is the default size used for the backing array
	// for transaction inputs and outputs.  The array will dynamically grow
	// as needed, but this figure is intended to provide enough space for
	// the number of inputs and outputs in a typical transaction without
	// needing to grow the backing array multiple times.
	defaultTxInOutAlloc = 15

	// minTxInPayload is the minimum payload size for a transaction input.
	// PreviousOutPoint.Hash + PreviousOutPoint.Index 4 bytes +
	// PreviousOutPoint.Tree 1 byte + Varint for SignatureScript length 1
	// byte + Sequence 4 bytes.
	minTxInPayload = 11 + chainhash.HashSize

	// maxTxInPerMessage is the maximum number of transactions inputs that
	// a transaction which fits into a message could possibly have.
	maxTxInPerMessage = (MaxMessagePayload / minTxInPayload) + 1

	// minTxOutPayload is the minimum payload size for a transaction output.
	// Value 8 bytes + Varint for PkScript length 1 byte.
	minTxOutPayload = 9

	// maxTxOutPerMessage is the maximum number of transactions outputs that
	// a transaction which fits into a message could possibly have.
	maxTxOutPerMessage = (MaxMessagePayload / minTxOutPayload) + 1

	// minTxPayload is the minimum payload size for any full encoded
	// (prefix and witness transaction). Note that any realistically
	// usable transaction must have at least one input or output, but
	// that is a rule enforced at a higher layer, so it is intentionally
	// not included here.
	// Version 4 bytes + Varint number of transaction inputs 1 byte + Varint
	// number of transaction outputs 1 byte + Varint representing the number
	// of transaction signatures + LockTime 4 bytes + Expiry 4 bytes + min
	// input payload + min output payload.
	minTxPayload = 4 + 1 + 1 + 1 + 4 + 4

	// freeListMaxScriptSize is the size of each buffer in the free list
	// that is used for deserializing scripts from the wire before they are
	// concatenated into a single contiguous buffer.  This value was chosen
	// because it is slightly more than twice the size of the vast majority
	// of all "standard" scripts.  Larger scripts are still deserialized
	// properly as the free list will simply be bypassed for them.
	freeListMaxScriptSize = 512

	// freeListMaxItems is the number of buffers to keep in the free list
	// to use for script deserialization.  This value allows up to 100
	// scripts per transaction being simultaneously deserialized by 125
	// peers.  Thus, the peak usage of the free list is 12,500 * 512 =
	// 6,400,000 bytes.
	freeListMaxItems = 12500
)

// TxSerializeType represents the serialized type of a transaction.
type TxSerializeType uint16

const (
	// TxSerializeFull indicates a transaction be serialized with the prefix
	// and all witness data.
	TxSerializeFull TxSerializeType = iota

	// TxSerializeNoWitness indicates a transaction be serialized with only
	// the prefix.
	TxSerializeNoWitness

	// TxSerializeOnlyWitness indicates a transaction be serialized with
	// only the witness data.
	TxSerializeOnlyWitness
)

// scriptFreeList defines a free list of byte slices (up to the maximum number
// defined by the freeListMaxItems constant) that have a cap according to the
// freeListMaxScriptSize constant.  It is used to provide temporary buffers for
// deserializing scripts in order to greatly reduce the number of allocations
// required.
//
// The caller can obtain a buffer from the free list by calling the Borrow
// function and should return it via the Return function when done using it.
type scriptFreeList chan []byte

// Borrow returns a byte slice from the free list with a length according the
// provided size.  A new buffer is allocated if there are any items available.
//
// When the size is larger than the max size allowed for items on the free list
// a new buffer of the appropriate size is allocated and returned.  It is safe
// to attempt to return said buffer via the Return function as it will be
// ignored and allowed to go the garbage collector.
func (c scriptFreeList) Borrow(size uint64) []byte {
	if size > freeListMaxScriptSize {
		return make([]byte, size)
	}

	var buf []byte
	select {
	case buf = <-c:
	default:
		buf = make([]byte, freeListMaxScriptSize)
	}
	return buf[:size]
}

// Return puts the provided byte slice back on the free list when it has a cap
// of the expected length.  The buffer is expected to have been obtained via
// the Borrow function.  Any slices that are not of the appropriate size, such
// as those whose size is greater than the largest allowed free list item size
// are simply ignored so they can go to the garbage collector.
func (c scriptFreeList) Return(buf []byte) {
	// Ignore any buffers returned that aren't the expected size for the
	// free list.
	if cap(buf) != freeListMaxScriptSize {
		return
	}

	// Return the buffer to the free list when it's not full.  Otherwise let
	// it be garbage collected.
	select {
	case c <- buf:
	default:
		// Let it go to the garbage collector.
	}
}

// Create the concurrent safe free list to use for script deserialization.  As
// previously described, this free list is maintained to significantly reduce
// the number of allocations.
var scriptPool scriptFreeList = make(chan []byte, freeListMaxItems)

// readScript reads a variable length byte array that represents a transaction
// script.  It is encoded as a varInt containing the length of the array
// followed by the bytes themselves.  An error is returned if the length is
// greater than the passed maxAllowed parameter which helps protect against
// memory exhaustion attacks and forced panics through malformed messages.  The
// fieldName parameter is only used for the error message so it provides more
// context in the error.
func readScript(r io.Reader, pver uint32, maxAllowed uint32, fieldName string) ([]byte, error) {
	const op = "readScript"
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return nil, err
	}

	// Prevent byte array larger than the max message size.  It would
	// be possible to cause memory exhaustion and panics without a sane
	// upper bound on this count.
	if count > uint64(maxAllowed) {
		msg := fmt.Sprintf("%s is larger than the max allowed size "+
			"[count %d, max %d]", fieldName, count, maxAllowed)
		return nil, messageError(op, ErrVarBytesTooLong, msg)
	}

	b := scriptPool.Borrow(count)
	_, err = io.ReadFull(r, b)
	if err != nil {
		scriptPool.Return(b)
		return nil, err
	}
	return b, nil
}

// OutPoint defines a Decred data type that is used to track previous
// transaction outputs.
type OutPoint struct {
	Hash  chainhash.Hash
	Index uint32
	Tree  int8
}

// NewOutPoint returns a new Decred transaction outpoint point with the
// provided hash and index.
func NewOutPoint(hash *chainhash.Hash, index uint32, tree int8) *OutPoint {
	return &OutPoint{
		Hash:  *hash,
		Index: index,
		Tree:  tree,
	}
}

// String returns the OutPoint in the human-readable form "hash:index".
func (o OutPoint) String() string {
	// Allocate enough for hash string, colon, and 10 digits.  Although
	// at the time of writing, the number of digits can be no greater than
	// the length of the decimal representation of maxTxOutPerMessage, the
	// maximum message payload may increase in the future and this
	// optimization may go unnoticed, so allocate space for 10 decimal
	// digits, which will fit any uint32.
	buf := make([]byte, 2*chainhash.HashSize+1, 2*chainhash.HashSize+1+10)
	copy(buf, o.Hash.String())
	buf[2*chainhash.HashSize] = ':'
	buf = strconv.AppendUint(buf, uint64(o.Index), 10)
	return string(buf)
}

// TxIn defines a Decred transaction input.
type TxIn struct {
	// Non-witness
	PreviousOutPoint OutPoint
	Sequence         uint32

	// Witness
	ValueIn         int64    // Value in atoms for VAR inputs
	SKAValueIn      *big.Int // Value in atoms for SKA inputs (nil for VAR)
	BlockHeight     uint32
	BlockIndex      uint32
	SignatureScript []byte
}

// SerializeSizePrefix returns the number of bytes it would take to serialize
// the transaction input for a prefix.
func (t *TxIn) SerializeSizePrefix() int {
	// Outpoint Hash 32 bytes + Outpoint Index 4 bytes + Outpoint Tree 1 byte +
	// Sequence 4 bytes.
	return 41
}

// SerializeSizeWitness returns the number of bytes it would take to serialize the
// transaction input for a witness.
// V13 format: [ValueIn:8][SKAValueInLen:1][SKAValueIn:N][BlockHeight:4][BlockIndex:4][SigScript:var]
func (t *TxIn) SerializeSizeWitness() int {
	// ValueIn (8 bytes) + SKAValueInLen (1 byte) + BlockHeight (4 bytes) +
	// BlockIndex (4 bytes) + serialized varint size for the length of
	// SignatureScript + SignatureScript bytes.
	base := 8 + 1 + 4 + 4 + VarIntSerializeSize(uint64(len(t.SignatureScript))) +
		len(t.SignatureScript)

	// Add SKAValueIn bytes if present
	if t.SKAValueIn != nil && t.SKAValueIn.Sign() > 0 {
		base += len(t.SKAValueIn.Bytes())
	}

	return base
}

// NewTxIn returns a new Decred transaction input with the provided
// previous outpoint point and signature script with a default sequence of
// MaxTxInSequenceNum.
func NewTxIn(prevOut *OutPoint, valueIn int64, signatureScript []byte) *TxIn {
	return &TxIn{
		PreviousOutPoint: *prevOut,
		Sequence:         MaxTxInSequenceNum,
		SignatureScript:  signatureScript,
		ValueIn:          valueIn,
		BlockHeight:      NullBlockHeight,
		BlockIndex:       NullBlockIndex,
	}
}

// TxOut defines a Decred transaction output.
type TxOut struct {
	Value    int64             // Value in atoms for VAR (always used for VAR)
	SKAValue *big.Int          // Value in atoms for SKA (nil for VAR outputs)
	CoinType cointype.CoinType // Coin type (VAR=0, SKA=1-255)
	Version  uint16
	PkScript []byte
}

// SerializeSize returns the number of bytes it would take to serialize the
// transaction output.
func (t *TxOut) SerializeSize() int {
	// CoinType 1 byte + Version 2 bytes + serialized varint size for
	// the length of PkScript + PkScript bytes.
	base := 1 + 2 + VarIntSerializeSize(uint64(len(t.PkScript))) + len(t.PkScript)

	// VAR: fixed 8-byte int64 value
	// SKA: 1-byte length prefix + variable-length big.Int bytes
	if t.CoinType.IsSKA() && t.SKAValue != nil {
		// SKA format: [CoinType:1][ValLen:1][Value:N bytes][Version:2][PkScript:var]
		valueBytes := t.SKAValue.Bytes()
		return base + 1 + len(valueBytes) // 1 byte for length prefix
	}

	// VAR format: [CoinType:1][Value:8 bytes][Version:2][PkScript:var]
	return base + 8
}

// NewTxOut returns a new Decred transaction output with the provided
// transaction value and public key script. For backward compatibility,
// this defaults to VAR coin type.
func NewTxOut(value int64, pkScript []byte) *TxOut {
	return NewTxOutWithCoinType(value, cointype.CoinTypeVAR, pkScript)
}

// NewTxOutWithCoinType returns a new Decred transaction output with the provided
// transaction value, coin type, and public key script.
// For VAR outputs, uses int64 value. For SKA outputs, use NewTxOutSKA instead.
func NewTxOutWithCoinType(value int64, coinType cointype.CoinType, pkScript []byte) *TxOut {
	return &TxOut{
		Value:    value,
		CoinType: coinType,
		Version:  DefaultPkScriptVersion,
		PkScript: pkScript,
	}
}

// NewTxOutSKA returns a new SKA transaction output with the provided
// big.Int value, SKA coin type, and public key script.
// The value is copied to ensure immutability.
func NewTxOutSKA(value *big.Int, coinType cointype.CoinType, pkScript []byte) *TxOut {
	var skaValue *big.Int
	if value != nil {
		skaValue = new(big.Int).Set(value)
	}
	return &TxOut{
		Value:    0, // Not used for SKA
		SKAValue: skaValue,
		CoinType: coinType,
		Version:  DefaultPkScriptVersion,
		PkScript: pkScript,
	}
}

// GetValue returns the output value as int64.
// For VAR outputs, returns Value directly.
// For SKA outputs, returns the int64 representation if it fits, otherwise 0.
// Use GetSKAValue for SKA outputs that may exceed int64.
func (t *TxOut) GetValue() int64 {
	if t.CoinType.IsSKA() && t.SKAValue != nil {
		if t.SKAValue.IsInt64() {
			return t.SKAValue.Int64()
		}
		return 0 // Overflow - caller should use GetSKAValue
	}
	return t.Value
}

// GetSKAValue returns the output value as *big.Int.
// For SKA outputs, returns a copy of SKAValue.
// For VAR outputs, converts Value to *big.Int.
func (t *TxOut) GetSKAValue() *big.Int {
	if t.CoinType.IsSKA() && t.SKAValue != nil {
		return new(big.Int).Set(t.SKAValue)
	}
	return big.NewInt(t.Value)
}

// ValidateCoinTypeFields checks that the TxOut uses the correct value field
// for its coin type. This prevents ambiguous outputs that could be exploited.
//
// Rules:
//   - VAR outputs: Must use Value (int64), SKAValue must be nil
//   - SKA outputs: Must use SKAValue (big.Int), Value must be 0
//
// Returns an error describing the violation, or nil if valid.
func (t *TxOut) ValidateCoinTypeFields() error {
	if t.CoinType.IsVAR() {
		// VAR outputs must not have SKAValue set
		if t.SKAValue != nil {
			return fmt.Errorf("VAR output has SKAValue set (must be nil)")
		}
		return nil
	}

	// SKA outputs must use SKAValue, not Value
	if t.Value != 0 {
		return fmt.Errorf("SKA output has Value=%d (must be 0, use SKAValue)", t.Value)
	}
	if t.SKAValue == nil {
		return fmt.Errorf("SKA output has nil SKAValue (must be set)")
	}
	if t.SKAValue.Sign() < 0 {
		return fmt.Errorf("SKA output has negative SKAValue: %s", t.SKAValue.String())
	}
	return nil
}

// MsgTx implements the Message interface and represents a Decred tx message.
// It is used to deliver transaction information in response to a getdata
// message (MsgGetData) for a given transaction.
//
// Use the AddTxIn and AddTxOut functions to build up the list of transaction
// inputs and outputs.
type MsgTx struct {
	CachedHash *chainhash.Hash
	SerType    TxSerializeType
	Version    uint16
	TxIn       []*TxIn
	TxOut      []*TxOut
	LockTime   uint32
	Expiry     uint32 // In blocks
}

// AddTxIn adds a transaction input to the message.
func (msg *MsgTx) AddTxIn(ti *TxIn) {
	msg.TxIn = append(msg.TxIn, ti)
}

// AddTxOut adds a transaction output to the message.
func (msg *MsgTx) AddTxOut(to *TxOut) {
	msg.TxOut = append(msg.TxOut, to)
}

// serialize returns the serialization of the transaction for the provided
// serialization type without modifying the original transaction.
func (msg *MsgTx) serialize(serType TxSerializeType) ([]byte, error) {
	// Shallow copy so the serialization type can be changed without
	// modifying the original transaction.
	mtxCopy := *msg
	mtxCopy.SerType = serType
	expectedSize := mtxCopy.SerializeSize()
	buf := bytes.NewBuffer(make([]byte, 0, expectedSize))
	err := mtxCopy.Serialize(buf)
	if err != nil {
		return nil, err
	}
	result := buf.Bytes()

	return result, nil
}

// mustSerialize returns the serialization of the transaction for the provided
// serialization type without modifying the original transaction. It logs
// critical errors and returns empty bytes instead of panicking if any errors occur.
func (msg *MsgTx) mustSerialize(serType TxSerializeType) []byte {
	serialized, err := msg.serialize(serType)
	if err != nil {
		log.Errorf("MsgTx failed serializing for type %v: %v", serType, err)
		log.Debugf("  Transaction details: Version=%d, SerType=%d, TxIn=%d, TxOut=%d",
			msg.Version, msg.SerType, len(msg.TxIn), len(msg.TxOut))
		if len(msg.TxOut) > 0 {
			log.Debugf("  First TxOut: CoinType=%d, Value=%d, Version=%d, PkScript len=%d",
				msg.TxOut[0].CoinType, msg.TxOut[0].Value, msg.TxOut[0].Version, len(msg.TxOut[0].PkScript))
		}
		log.Debugf("  Expected SerializeSize: %d", msg.SerializeSize())
		panic(fmt.Sprintf("MsgTx failed serializing for type %v", serType))
	}
	if len(serialized) == 0 {
		log.Warnf("Successful serialization produced empty bytes for type %v", serType)
	}
	return serialized
}

// TxHash generates the hash for the transaction prefix.  Since it does not
// contain any witness data, it is not malleable and therefore is stable for
// use in unconfirmed transaction chains.
func (msg *MsgTx) TxHash() chainhash.Hash {
	// TxHash should always calculate a non-witnessed hash.
	return chainhash.HashH(msg.mustSerialize(TxSerializeNoWitness))
}

// CachedTxHash is equivalent to calling TxHash, however it caches the result so
// subsequent calls do not have to recalculate the hash.  It can be recalculated
// later with RecacheTxHash.
func (msg *MsgTx) CachedTxHash() *chainhash.Hash {
	if msg.CachedHash == nil {
		h := msg.TxHash()
		msg.CachedHash = &h
	}

	return msg.CachedHash
}

// RecacheTxHash is equivalent to calling TxHash, however it replaces the cached
// result so future calls to CachedTxHash will return this newly calculated
// hash.
func (msg *MsgTx) RecacheTxHash() *chainhash.Hash {
	h := msg.TxHash()
	msg.CachedHash = &h

	return msg.CachedHash
}

// TxHashWitness generates the hash for the transaction witness.
func (msg *MsgTx) TxHashWitness() chainhash.Hash {
	// TxHashWitness should always calculate a witnessed hash.
	return chainhash.HashH(msg.mustSerialize(TxSerializeOnlyWitness))
}

// TxHashFull generates the hash for the transaction prefix || witness. It first
// obtains the hashes for both the transaction prefix and witness, then
// concatenates them and hashes the result.
func (msg *MsgTx) TxHashFull() chainhash.Hash {
	// Note that the inputs to the hashes, the serialized prefix and
	// witness, have different serialized versions because the serialized
	// encoding of the version includes the real transaction version in the
	// lower 16 bits and the transaction serialization type in the upper 16
	// bits.  The real transaction version (lower 16 bits) will be the same
	// in both serializations.
	concat := make([]byte, chainhash.HashSize*2)
	prefixHash := msg.TxHash()
	witnessHash := msg.TxHashWitness()

	copy(concat[0:], prefixHash[:])
	copy(concat[chainhash.HashSize:], witnessHash[:])
	fullHash := chainhash.HashH(concat)

	return fullHash
}

// Copy creates a deep copy of a transaction so that the original does not get
// modified when the copy is manipulated.
func (msg *MsgTx) Copy() *MsgTx {
	// Create new tx and start by copying primitive values and making space
	// for the transaction inputs and outputs.
	newTx := MsgTx{
		SerType:  msg.SerType,
		Version:  msg.Version,
		TxIn:     make([]*TxIn, 0, len(msg.TxIn)),
		TxOut:    make([]*TxOut, 0, len(msg.TxOut)),
		LockTime: msg.LockTime,
		Expiry:   msg.Expiry,
	}

	// Deep copy the old TxIn data.
	for _, oldTxIn := range msg.TxIn {
		// Deep copy the old previous outpoint.
		oldOutPoint := oldTxIn.PreviousOutPoint
		newOutPoint := OutPoint{}
		newOutPoint.Hash.SetBytes(oldOutPoint.Hash[:])
		newOutPoint.Index = oldOutPoint.Index
		newOutPoint.Tree = oldOutPoint.Tree

		// Deep copy the old signature script.
		var newScript []byte
		oldScript := oldTxIn.SignatureScript
		oldScriptLen := len(oldScript)
		if oldScriptLen > 0 {
			newScript = make([]byte, oldScriptLen)
			copy(newScript, oldScript[:oldScriptLen])
		}

		// Deep copy SKAValueIn if present
		var newSKAValueIn *big.Int
		if oldTxIn.SKAValueIn != nil {
			newSKAValueIn = new(big.Int).Set(oldTxIn.SKAValueIn)
		}

		// Create new txIn with the deep copied data and append it to
		// new Tx.
		newTxIn := TxIn{
			PreviousOutPoint: newOutPoint,
			Sequence:         oldTxIn.Sequence,
			ValueIn:          oldTxIn.ValueIn,
			SKAValueIn:       newSKAValueIn,
			BlockHeight:      oldTxIn.BlockHeight,
			BlockIndex:       oldTxIn.BlockIndex,
			SignatureScript:  newScript,
		}
		newTx.TxIn = append(newTx.TxIn, &newTxIn)
	}

	// Deep copy the old TxOut data.
	for _, oldTxOut := range msg.TxOut {
		// Deep copy the old PkScript
		var newScript []byte
		oldScript := oldTxOut.PkScript
		oldScriptLen := len(oldScript)
		if oldScriptLen > 0 {
			newScript = make([]byte, oldScriptLen)
			copy(newScript, oldScript[:oldScriptLen])
		}

		// Deep copy SKAValue if present
		var newSKAValue *big.Int
		if oldTxOut.SKAValue != nil {
			newSKAValue = new(big.Int).Set(oldTxOut.SKAValue)
		}

		// Create new txOut with the deep copied data and append it to
		// new Tx.
		newTxOut := TxOut{
			Value:    oldTxOut.Value,
			SKAValue: newSKAValue,
			CoinType: oldTxOut.CoinType,
			Version:  oldTxOut.Version,
			PkScript: newScript,
		}
		newTx.TxOut = append(newTx.TxOut, &newTxOut)
	}

	return &newTx
}

// writeTxScriptsToMsgTx allocates the memory for variable length fields in a
// MsgTx TxIns, TxOuts, or both as a contiguous chunk of memory, then fills
// in these fields for the MsgTx by copying to a contiguous piece of memory
// and setting the pointer.
//
// NOTE: It is no longer valid to return any previously borrowed script
// buffers after this function has run because it is already done and the
// scripts in the transaction inputs and outputs no longer point to the
// buffers.
func writeTxScriptsToMsgTx(msg *MsgTx, totalScriptSize uint64, serType TxSerializeType) {
	// Create a single allocation to house all of the scripts and set each
	// input signature scripts and output public key scripts to the
	// appropriate subslice of the overall contiguous buffer.  Then, return
	// each individual script buffer back to the pool so they can be reused
	// for future deserializations.  This is done because it significantly
	// reduces the number of allocations the garbage collector needs to track,
	// which in turn improves performance and drastically reduces the amount
	// of runtime overhead that would otherwise be needed to keep track of
	// millions of small allocations.
	//
	// Closures around writing the TxIn and TxOut scripts are used in Decred
	// because, depending on the serialization type desired, only input or
	// output scripts may be required.
	var offset uint64
	scripts := make([]byte, totalScriptSize)
	writeTxIns := func() {
		for i := 0; i < len(msg.TxIn); i++ {
			// Copy the signature script into the contiguous buffer at the
			// appropriate offset.
			signatureScript := msg.TxIn[i].SignatureScript
			copy(scripts[offset:], signatureScript)

			// Reset the signature script of the transaction input to the
			// slice of the contiguous buffer where the script lives.
			scriptSize := uint64(len(signatureScript))
			end := offset + scriptSize
			msg.TxIn[i].SignatureScript = scripts[offset:end:end]
			offset += scriptSize

			// Return the temporary script buffer to the pool.
			scriptPool.Return(signatureScript)
		}
	}
	writeTxOuts := func() {
		for i := 0; i < len(msg.TxOut); i++ {
			// Copy the public key script into the contiguous buffer at the
			// appropriate offset.
			pkScript := msg.TxOut[i].PkScript
			copy(scripts[offset:], pkScript)

			// Reset the public key script of the transaction output to the
			// slice of the contiguous buffer where the script lives.
			scriptSize := uint64(len(pkScript))
			end := offset + scriptSize
			msg.TxOut[i].PkScript = scripts[offset:end:end]
			offset += scriptSize

			// Return the temporary script buffer to the pool.
			scriptPool.Return(pkScript)
		}
	}

	// Handle the serialization types accordingly.
	switch serType {
	case TxSerializeNoWitness:
		writeTxOuts()
	case TxSerializeOnlyWitness:
		fallthrough
	case TxSerializeFull:
		writeTxIns()
		writeTxOuts()
	}
}

// decodePrefix decodes a transaction prefix and stores the contents
// in the embedded msgTx.
func (msg *MsgTx) decodePrefix(r io.Reader, pver uint32) (uint64, error) {
	const op = "MsgTx.decodePrefix"
	count, err := ReadVarInt(r, pver)
	if err != nil {
		return 0, err
	}

	// Prevent more input transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(maxTxInPerMessage) {
		msg := fmt.Sprintf("too many input transactions to fit into max "+
			"message size [count %d, max %d]", count, maxTxInPerMessage)
		return 0, messageError(op, ErrTooManyTxs, msg)
	}

	// TxIns.
	txIns := make([]TxIn, count)
	msg.TxIn = make([]*TxIn, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		ti := &txIns[i]
		msg.TxIn[i] = ti
		err = readTxInPrefix(r, pver, msg.SerType, msg.Version, ti)
		if err != nil {
			return 0, err
		}
	}

	count, err = ReadVarInt(r, pver)
	if err != nil {
		return 0, err
	}

	// Prevent more output transactions than could possibly fit into a
	// message.  It would be possible to cause memory exhaustion and panics
	// without a sane upper bound on this count.
	if count > uint64(maxTxOutPerMessage) {
		msg := fmt.Sprintf("too many output transactions to fit into "+
			"max message size [count %d, max %d]", count, maxTxOutPerMessage)
		return 0, messageError(op, ErrTooManyTxs, msg)
	}

	// TxOuts.
	var totalScriptSize uint64
	txOuts := make([]TxOut, count)
	msg.TxOut = make([]*TxOut, count)
	for i := uint64(0); i < count; i++ {
		// The pointer is set now in case a script buffer is borrowed
		// and needs to be returned to the pool on error.
		to := &txOuts[i]
		msg.TxOut[i] = to
		err = readTxOut(r, pver, msg.Version, to)
		if err != nil {
			return 0, err
		}
		totalScriptSize += uint64(len(to.PkScript))
	}

	// Locktime and expiry.
	msg.LockTime, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return 0, err
	}

	msg.Expiry, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return 0, err
	}

	return totalScriptSize, nil
}

func (msg *MsgTx) decodeWitness(r io.Reader, pver uint32, isFull bool) (uint64, error) {
	const op = "MsgTx.decodeWitness"

	// Witness only; generate the TxIn list and fill out only the
	// sigScripts.
	var totalScriptSize uint64
	if !isFull {
		count, err := ReadVarInt(r, pver)
		if err != nil {
			return 0, err
		}

		// Prevent more input transactions than could possibly fit into a
		// message.  It would be possible to cause memory exhaustion and panics
		// without a sane upper bound on this count.
		if count > uint64(maxTxInPerMessage) {
			str := fmt.Sprintf("too many input transactions to fit into "+
				"max message size [count %d, max %d]", count, maxTxInPerMessage)
			return 0, messageError(op, ErrTooManyTxs, str)
		}

		txIns := make([]TxIn, count)
		msg.TxIn = make([]*TxIn, count)
		for i := uint64(0); i < count; i++ {
			// The pointer is set now in case a script buffer is borrowed
			// and needs to be returned to the pool on error.
			ti := &txIns[i]
			msg.TxIn[i] = ti
			err = readTxInWitness(r, pver, msg.Version, ti)
			if err != nil {
				return 0, err
			}
			totalScriptSize += uint64(len(ti.SignatureScript))
		}
		msg.TxOut = make([]*TxOut, 0)
	} else {
		// We're decoding witnesses from a full transaction, so read in
		// the number of signature scripts, check to make sure it's the
		// same as the number of TxIns we currently have, then fill in
		// the signature scripts.
		count, err := ReadVarInt(r, pver)
		if err != nil {
			return 0, err
		}

		// Don't allow the deserializer to panic by accessing memory
		// that doesn't exist.
		if int(count) != len(msg.TxIn) {
			msg := fmt.Sprintf("non equal witness and prefix txin quantities "+
				"(witness %v, prefix %v)", count, len(msg.TxIn))
			return 0, messageError(op, ErrMismatchedWitnessCount, msg)
		}

		// Prevent more input transactions than could possibly fit into a
		// message.  It would be possible to cause memory exhaustion and panics
		// without a sane upper bound on this count.
		if count > uint64(maxTxInPerMessage) {
			msg := fmt.Sprintf("too many input transactions to fit into "+
				"max message size [count %d, max %d]", count, maxTxInPerMessage)
			return 0, messageError(op, ErrTooManyTxs, msg)
		}

		// Read in the witnesses, and copy them into the already generated
		// by decodePrefix TxIns.
		txIns := make([]TxIn, count)
		for i := uint64(0); i < count; i++ {
			ti := &txIns[i]
			err = readTxInWitness(r, pver, msg.Version, ti)
			if err != nil {
				return 0, err
			}
			totalScriptSize += uint64(len(ti.SignatureScript))

			msg.TxIn[i].ValueIn = ti.ValueIn
			msg.TxIn[i].SKAValueIn = ti.SKAValueIn // V13: Copy SKA value for dual-coin support
			msg.TxIn[i].BlockHeight = ti.BlockHeight
			msg.TxIn[i].BlockIndex = ti.BlockIndex
			msg.TxIn[i].SignatureScript = ti.SignatureScript
		}
	}

	return totalScriptSize, nil
}

// BtcDecode decodes r using the Decred protocol encoding into the receiver.
// This is part of the Message interface implementation.
// See Deserialize for decoding transactions stored to disk, such as in a
// database, as opposed to decoding transactions from the wire.
func (msg *MsgTx) BtcDecode(r io.Reader, pver uint32) error {
	const op = "MsgTx.BtcDecode"

	// The serialized encoding of the version includes the real transaction
	// version in the lower 16 bits and the transaction serialization type
	// in the upper 16 bits.
	version, err := binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}
	msg.Version = uint16(version & 0xffff)
	msg.SerType = TxSerializeType(version >> 16)

	// returnScriptBuffers is a closure that returns any script buffers that
	// were borrowed from the pool when there are any deserialization
	// errors.  This is only valid to call before the final step which
	// replaces the scripts with the location in a contiguous buffer and
	// returns them.
	returnScriptBuffers := func() {
		for _, txIn := range msg.TxIn {
			if txIn == nil || txIn.SignatureScript == nil {
				continue
			}
			scriptPool.Return(txIn.SignatureScript)
		}
		for _, txOut := range msg.TxOut {
			if txOut == nil || txOut.PkScript == nil {
				continue
			}
			scriptPool.Return(txOut.PkScript)
		}
	}

	// Serialize the transactions depending on their serialization
	// types.  Write the transaction scripts at the end of each
	// serialization procedure using the more efficient contiguous
	// memory allocations, which reduces the amount of memory that
	// must be handled by the GC tremendously.  If any of these
	// serializations fail, free the relevant memory.
	switch txSerType := msg.SerType; txSerType {
	case TxSerializeNoWitness:
		totalScriptSize, err := msg.decodePrefix(r, pver)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		writeTxScriptsToMsgTx(msg, totalScriptSize, txSerType)

	case TxSerializeOnlyWitness:
		totalScriptSize, err := msg.decodeWitness(r, pver, false)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		writeTxScriptsToMsgTx(msg, totalScriptSize, txSerType)

	case TxSerializeFull:
		totalScriptSizeIns, err := msg.decodePrefix(r, pver)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		totalScriptSizeOuts, err := msg.decodeWitness(r, pver, true)
		if err != nil {
			returnScriptBuffers()
			return err
		}
		writeTxScriptsToMsgTx(msg, totalScriptSizeIns+
			totalScriptSizeOuts, txSerType)

	default:
		return messageError(op, ErrUnknownTxType, "unsupported transaction type")
	}

	return nil
}

// Deserialize decodes a transaction from r into the receiver using a format
// that is suitable for long-term storage such as a database while respecting
// the Version field in the transaction.  This function differs from BtcDecode
// in that BtcDecode decodes from the Decred wire protocol as it was sent
// across the network.  The wire encoding can technically differ depending on
// the protocol version and doesn't even really need to match the format of a
// stored transaction at all.  As of the time this comment was written, the
// encoded transaction is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
//
// This function uses the current protocol version (v13 SKABigIntVersion) which
// puts CoinType first in TxOut serialization to support variable-length SKA amounts.
// This is consistent with Serialize() which also uses ProtocolVersion.
func (msg *MsgTx) Deserialize(r io.Reader) error {
	return msg.BtcDecode(r, ProtocolVersion)
}

// FromBytes deserializes a transaction byte slice.
func (msg *MsgTx) FromBytes(b []byte) error {
	r := bytes.NewReader(b)
	return msg.Deserialize(r)
}

// encodePrefix encodes a transaction prefix into a writer.
func (msg *MsgTx) encodePrefix(w io.Writer, pver uint32) error {
	count := uint64(len(msg.TxIn))
	err := WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, ti := range msg.TxIn {
		err = writeTxInPrefix(w, pver, msg.Version, ti)
		if err != nil {
			return err
		}
	}

	count = uint64(len(msg.TxOut))
	err = WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, to := range msg.TxOut {
		err = writeTxOut(w, pver, msg.Version, to)
		if err != nil {
			return err
		}
	}

	err = binarySerializer.PutUint32(w, littleEndian, msg.LockTime)
	if err != nil {
		return err
	}

	return binarySerializer.PutUint32(w, littleEndian, msg.Expiry)
}

// encodeWitness encodes a transaction witness into a writer.
func (msg *MsgTx) encodeWitness(w io.Writer, pver uint32) error {
	count := uint64(len(msg.TxIn))
	err := WriteVarInt(w, pver, count)
	if err != nil {
		return err
	}

	for _, ti := range msg.TxIn {
		err = writeTxInWitness(w, pver, msg.Version, ti)
		if err != nil {
			return err
		}
	}

	return nil
}

// BtcEncode encodes the receiver to w using the Decred protocol encoding.
// This is part of the Message interface implementation.
// See Serialize for encoding transactions to be stored to disk, such as in a
// database, as opposed to encoding transactions for the wire.
func (msg *MsgTx) BtcEncode(w io.Writer, pver uint32) error {
	// The serialized encoding of the version includes the real transaction
	// version in the lower 16 bits and the transaction serialization type
	// in the upper 16 bits.
	serializedVersion := uint32(msg.Version) | uint32(msg.SerType)<<16
	err := binarySerializer.PutUint32(w, littleEndian, serializedVersion)
	if err != nil {
		return err
	}

	switch msg.SerType {
	case TxSerializeNoWitness:
		err := msg.encodePrefix(w, pver)
		if err != nil {
			return err
		}

	case TxSerializeOnlyWitness:
		err := msg.encodeWitness(w, pver)
		if err != nil {
			return err
		}

	case TxSerializeFull:
		err := msg.encodePrefix(w, pver)
		if err != nil {
			return err
		}
		err = msg.encodeWitness(w, pver)
		if err != nil {
			return err
		}

	default:
		return messageError("MsgTx.BtcEncode", ErrUnknownTxType,
			"unsupported transaction type")
	}

	return nil
}

// Serialize encodes the transaction to w using a format that suitable for
// long-term storage such as a database while respecting the Version field in
// the transaction.  This function differs from BtcEncode in that BtcEncode
// encodes the transaction to the Decred wire protocol in order to be sent
// across the network.  The wire encoding can technically differ depending on
// the protocol version and doesn't even really need to match the format of a
// stored transaction at all.  As of the time this comment was written, the
// encoded transaction is the same in both instances, but there is a distinct
// difference and separating the two allows the API to be flexible enough to
// deal with changes.
func (msg *MsgTx) Serialize(w io.Writer) error {
	// Use current protocol version which includes dual-coin support with
	// CoinType first in TxOut serialization.
	return msg.BtcEncode(w, ProtocolVersion)
}

// Bytes returns the serialized form of the transaction in bytes.
func (msg *MsgTx) Bytes() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, msg.SerializeSize()))
	err := msg.Serialize(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BytesPrefix returns the serialized form of the transaction prefix in bytes.
func (msg *MsgTx) BytesPrefix() ([]byte, error) {
	return msg.serialize(TxSerializeNoWitness)
}

// BytesWitness returns the serialized form of the transaction prefix in bytes.
func (msg *MsgTx) BytesWitness() ([]byte, error) {
	return msg.serialize(TxSerializeOnlyWitness)
}

// SerializeSize returns the number of bytes it would take to serialize the
// transaction.
func (msg *MsgTx) SerializeSize() int {
	// Unknown type return 0.
	n := 0
	switch msg.SerType {
	case TxSerializeNoWitness:
		// Version 4 bytes + LockTime 4 bytes + Expiry 4 bytes +
		// Serialized varint size for the number of transaction
		// inputs and outputs.
		n = 12 + VarIntSerializeSize(uint64(len(msg.TxIn))) +
			VarIntSerializeSize(uint64(len(msg.TxOut)))

		for _, txIn := range msg.TxIn {
			n += txIn.SerializeSizePrefix()
		}
		for _, txOut := range msg.TxOut {
			n += txOut.SerializeSize()
		}

	case TxSerializeOnlyWitness:
		// Version 4 bytes + Serialized varint size for the
		// number of transaction signatures.
		n = 4 + VarIntSerializeSize(uint64(len(msg.TxIn)))

		for _, txIn := range msg.TxIn {
			n += txIn.SerializeSizeWitness()
		}

	case TxSerializeFull:
		// Version 4 bytes + LockTime 4 bytes + Expiry 4 bytes + Serialized
		// varint size for the number of transaction inputs (x2) and
		// outputs. The number of inputs is added twice because it's
		// encoded once in both the witness and the prefix.
		n = 12 + VarIntSerializeSize(uint64(len(msg.TxIn))) +
			VarIntSerializeSize(uint64(len(msg.TxIn))) +
			VarIntSerializeSize(uint64(len(msg.TxOut)))

		for _, txIn := range msg.TxIn {
			n += txIn.SerializeSizePrefix()
		}
		for _, txIn := range msg.TxIn {
			n += txIn.SerializeSizeWitness()
		}
		for _, txOut := range msg.TxOut {
			n += txOut.SerializeSize()
		}
	}

	return n
}

// Command returns the protocol command string for the message.  This is part
// of the Message interface implementation.
func (msg *MsgTx) Command() string {
	return CmdTx
}

// MaxPayloadLength returns the maximum length the payload can be for the
// receiver.  This is part of the Message interface implementation.
func (msg *MsgTx) MaxPayloadLength(pver uint32) uint32 {
	// Protocol version 3 and lower have a different max block payload.
	if pver <= 3 {
		return MaxBlockPayloadV3
	}

	return MaxBlockPayload
}

// PkScriptLocs returns a slice containing the start of each public key script
// within the raw serialized transaction.  The caller can easily obtain the
// length of each script by using len on the script available via the
// appropriate transaction output entry.
// TODO: Make this work for all serialization types, not just the full
// serialization type.
func (msg *MsgTx) PkScriptLocs() []int {
	// Return nil for witness-only tx.
	numTxOut := len(msg.TxOut)
	if numTxOut == 0 {
		return nil
	}

	// The starting offset in the serialized transaction of the first
	// transaction output is:
	//
	// Version 4 bytes + serialized varint size for the number of
	// transaction inputs and outputs + serialized size of each transaction
	// input.
	n := 4 + VarIntSerializeSize(uint64(len(msg.TxIn))) +
		VarIntSerializeSize(uint64(numTxOut))
	for _, txIn := range msg.TxIn {
		n += txIn.SerializeSizePrefix()
	}

	// Calculate and set the appropriate offset for each public key script.
	// The offset depends on the coin type (VAR vs SKA) as they have different
	// wire formats in protocol version 13+.
	pkScriptLocs := make([]int, numTxOut)
	for i, txOut := range msg.TxOut {
		// The offset of the script in the transaction output is:
		// CoinType 1 byte + value size (varies) + version 2 bytes +
		// serialized varint size for the length of PkScript.
		//
		// For VAR: CoinType(1) + Value(8) + Version(2) + VarInt + PkScript
		// For SKA: CoinType(1) + ValLen(1) + Value(N) + Version(2) + VarInt + PkScript
		var valueSize int
		if txOut.CoinType.IsSKA() && txOut.SKAValue != nil {
			// SKA: 1 byte length prefix + variable bytes
			valueBytes := txOut.SKAValue.Bytes()
			valueSize = 1 + len(valueBytes)
		} else {
			// VAR: fixed 8 bytes
			valueSize = 8
		}
		n += 1 + valueSize + 2 + VarIntSerializeSize(uint64(len(txOut.PkScript)))
		pkScriptLocs[i] = n
		n += len(txOut.PkScript)
	}

	return pkScriptLocs
}

// NewMsgTx returns a new Decred tx message that conforms to the Message
// interface.  The return instance has a default version of TxVersion and there
// are no transaction inputs or outputs.  Also, the lock time is set to zero
// to indicate the transaction is valid immediately as opposed to some time in
// future.
func NewMsgTx() *MsgTx {
	return &MsgTx{
		SerType: TxSerializeFull,
		Version: TxVersion,
		TxIn:    make([]*TxIn, 0, defaultTxInOutAlloc),
		TxOut:   make([]*TxOut, 0, defaultTxInOutAlloc),
	}
}

// ReadOutPoint reads the next sequence of bytes from r as an OutPoint.
func ReadOutPoint(r io.Reader, pver uint32, version uint16, op *OutPoint) error {
	_, err := io.ReadFull(r, op.Hash[:])
	if err != nil {
		return err
	}

	op.Index, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}

	tree, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}
	op.Tree = int8(tree)

	return nil
}

// WriteOutPoint encodes op to the Decred protocol encoding for an OutPoint
// to w.
func WriteOutPoint(w io.Writer, pver uint32, version uint16, op *OutPoint) error {
	_, err := w.Write(op.Hash[:])
	if err != nil {
		return err
	}

	err = binarySerializer.PutUint32(w, littleEndian, op.Index)
	if err != nil {
		return err
	}

	return binarySerializer.PutUint8(w, uint8(op.Tree))
}

// readTxInPrefix reads the next sequence of bytes from r as a transaction input
// (TxIn) in the transaction prefix.
func readTxInPrefix(r io.Reader, pver uint32, serType TxSerializeType, version uint16, ti *TxIn) error {
	if serType == TxSerializeOnlyWitness {
		return messageError("readTxInPrefix", ErrReadInPrefixFromWitnessOnlyTx,
			"tried to read a prefix input for a witness only tx")
	}

	// Outpoint.
	err := ReadOutPoint(r, pver, version, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	// Sequence.
	ti.Sequence, err = binarySerializer.Uint32(r, littleEndian)
	return err
}

// readTxInWitness reads the next sequence of bytes from r as a transaction input
// (TxIn) in the transaction witness.
func readTxInWitness(r io.Reader, pver uint32, version uint16, ti *TxIn) error {
	// SKABigIntVersion introduces SKAValueIn support for inputs
	if pver >= SKABigIntVersion {
		return readTxInWitnessV13(r, pver, version, ti)
	}

	// ValueIn.
	valueIn, err := binarySerializer.Uint64(r, littleEndian)
	if err != nil {
		return err
	}
	ti.ValueIn = int64(valueIn)

	// BlockHeight.
	ti.BlockHeight, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}

	// BlockIndex.
	ti.BlockIndex, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}

	// Signature script.
	ti.SignatureScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction input signature script")
	return err
}

// readTxInWitnessV13 reads a TxIn witness using the SKABigIntVersion (v13) wire format.
// Format:
//
//	[ValueIn:8 bytes][SKAValueInLen:1 byte][SKAValueIn:N bytes][BlockHeight:4][BlockIndex:4][SigScript:var]
//
// SKAValueInLen is 0 for VAR inputs or inputs without SKA value set.
func readTxInWitnessV13(r io.Reader, pver uint32, version uint16, ti *TxIn) error {
	// ValueIn (always present for fraud proofs)
	valueIn, err := binarySerializer.Uint64(r, littleEndian)
	if err != nil {
		return err
	}
	ti.ValueIn = int64(valueIn)

	// SKAValueIn length (0 = no SKA value)
	skaValueLen, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}

	if skaValueLen > 0 {
		// Read SKAValueIn bytes
		valueBytes := make([]byte, skaValueLen)
		_, err = io.ReadFull(r, valueBytes)
		if err != nil {
			return err
		}
		ti.SKAValueIn = new(big.Int).SetBytes(valueBytes)
	} else {
		ti.SKAValueIn = nil
	}

	// BlockHeight.
	ti.BlockHeight, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}

	// BlockIndex.
	ti.BlockIndex, err = binarySerializer.Uint32(r, littleEndian)
	if err != nil {
		return err
	}

	// Signature script.
	ti.SignatureScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction input signature script")
	return err
}

// writeTxInPrefix encodes ti to the Decred protocol encoding for a transaction
// input (TxIn) prefix to w.
func writeTxInPrefix(w io.Writer, pver uint32, version uint16, ti *TxIn) error {
	err := WriteOutPoint(w, pver, version, &ti.PreviousOutPoint)
	if err != nil {
		return err
	}

	return binarySerializer.PutUint32(w, littleEndian, ti.Sequence)
}

// writeTxInWitness encodes ti to the Decred protocol encoding for a transaction
// input (TxIn) witness to w.
func writeTxInWitness(w io.Writer, pver uint32, version uint16, ti *TxIn) error {
	// SKABigIntVersion introduces SKAValueIn support for inputs
	if pver >= SKABigIntVersion {
		return writeTxInWitnessV13(w, pver, version, ti)
	}

	// ValueIn.
	err := binarySerializer.PutUint64(w, littleEndian, uint64(ti.ValueIn))
	if err != nil {
		return err
	}

	// BlockHeight.
	err = binarySerializer.PutUint32(w, littleEndian, ti.BlockHeight)
	if err != nil {
		return err
	}

	// BlockIndex.
	err = binarySerializer.PutUint32(w, littleEndian, ti.BlockIndex)
	if err != nil {
		return err
	}

	// Write the signature script.
	return WriteVarBytes(w, pver, ti.SignatureScript)
}

// writeTxInWitnessV13 writes a TxIn witness using the SKABigIntVersion (v13) wire format.
// Format:
//
//	[ValueIn:8 bytes][SKAValueInLen:1 byte][SKAValueIn:N bytes][BlockHeight:4][BlockIndex:4][SigScript:var]
//
// SKAValueInLen is 0 for VAR inputs or inputs without SKA value set.
func writeTxInWitnessV13(w io.Writer, pver uint32, version uint16, ti *TxIn) error {
	// ValueIn (always present for fraud proofs)
	err := binarySerializer.PutUint64(w, littleEndian, uint64(ti.ValueIn))
	if err != nil {
		return err
	}

	// SKAValueIn - write length prefix + bytes (or 0 for no SKA value)
	var skaValueBytes []byte
	if ti.SKAValueIn != nil && ti.SKAValueIn.Sign() > 0 {
		skaValueBytes = ti.SKAValueIn.Bytes()
	}

	if len(skaValueBytes) == 0 {
		// No SKA value: write length 0
		err = binarySerializer.PutUint8(w, 0)
		if err != nil {
			return err
		}
	} else {
		// Write length prefix + big-endian bytes
		if len(skaValueBytes) > 255 {
			return messageError("writeTxInWitnessV13", ErrVarBytesTooLong,
				"SKA value exceeds maximum length of 255 bytes")
		}
		err = binarySerializer.PutUint8(w, uint8(len(skaValueBytes)))
		if err != nil {
			return err
		}
		_, err = w.Write(skaValueBytes)
		if err != nil {
			return err
		}
	}

	// BlockHeight.
	err = binarySerializer.PutUint32(w, littleEndian, ti.BlockHeight)
	if err != nil {
		return err
	}

	// BlockIndex.
	err = binarySerializer.PutUint32(w, littleEndian, ti.BlockIndex)
	if err != nil {
		return err
	}

	// Write the signature script.
	return WriteVarBytes(w, pver, ti.SignatureScript)
}

// readTxOut reads the next sequence of bytes from r as a transaction output
// (TxOut).
func readTxOut(r io.Reader, pver uint32, version uint16, to *TxOut) error {
	// SKABigIntVersion introduces a new wire format where CoinType comes first
	// to determine the value format.
	if pver >= SKABigIntVersion {
		return readTxOutV13(r, pver, version, to)
	}

	// Legacy format: Value first, then CoinType
	value, err := binarySerializer.Uint64(r, littleEndian)
	if err != nil {
		return err
	}
	to.Value = int64(value)

	// CoinType field was added in DualCoinVersion
	if pver >= DualCoinVersion {
		coinType, err := binarySerializer.Uint8(r)
		if err != nil {
			return err
		}
		to.CoinType = cointype.CoinType(coinType)
	} else {
		// Default to VAR for backward compatibility
		to.CoinType = cointype.CoinTypeVAR
	}

	to.Version, err = binarySerializer.Uint16(r, littleEndian)
	if err != nil {
		return err
	}

	to.PkScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction output public key script")
	return err
}

// readTxOutV13 reads a TxOut using the SKABigIntVersion (v13) wire format.
// Format:
//
//	VAR: [CoinType:1][Value:8 bytes][Version:2][PkScript:var]
//	SKA: [CoinType:1][ValLen:1][Value:N bytes][Version:2][PkScript:var]
func readTxOutV13(r io.Reader, pver uint32, version uint16, to *TxOut) error {
	// Read CoinType first to determine value format
	coinType, err := binarySerializer.Uint8(r)
	if err != nil {
		return err
	}
	to.CoinType = cointype.CoinType(coinType)

	// Read value based on coin type
	if to.CoinType.IsSKA() {
		// SKA: variable-length big.Int with 1-byte length prefix
		valueLen, err := binarySerializer.Uint8(r)
		if err != nil {
			return err
		}

		if valueLen == 0 {
			// Zero value
			to.SKAValue = new(big.Int)
		} else {
			// Read the value bytes
			valueBytes := make([]byte, valueLen)
			_, err = io.ReadFull(r, valueBytes)
			if err != nil {
				return err
			}
			to.SKAValue = new(big.Int).SetBytes(valueBytes)
		}
		to.Value = 0 // Not used for SKA
	} else {
		// VAR: fixed 8-byte int64 little-endian
		value, err := binarySerializer.Uint64(r, littleEndian)
		if err != nil {
			return err
		}
		to.Value = int64(value)
		to.SKAValue = nil
	}

	// Version
	to.Version, err = binarySerializer.Uint16(r, littleEndian)
	if err != nil {
		return err
	}

	// PkScript
	to.PkScript, err = readScript(r, pver, MaxMessagePayload,
		"transaction output public key script")
	return err
}

// writeTxOut encodes to into the Decred protocol encoding for a transaction
// output (TxOut) to w.
func writeTxOut(w io.Writer, pver uint32, version uint16, to *TxOut) error {
	// SKABigIntVersion introduces a new wire format where CoinType comes first
	// to determine the value format.
	if pver >= SKABigIntVersion {
		return writeTxOutV13(w, pver, version, to)
	}

	// Legacy format: Value first, then CoinType
	err := binarySerializer.PutUint64(w, littleEndian, uint64(to.Value))
	if err != nil {
		return err
	}

	// CoinType field was added in DualCoinVersion
	if pver >= DualCoinVersion {
		err = binarySerializer.PutUint8(w, uint8(to.CoinType))
		if err != nil {
			return err
		}
	}

	err = binarySerializer.PutUint16(w, littleEndian, to.Version)
	if err != nil {
		return err
	}

	return WriteVarBytes(w, pver, to.PkScript)
}

// writeTxOutV13 writes a TxOut using the SKABigIntVersion (v13) wire format.
// Format:
//
//	VAR: [CoinType:1][Value:8 bytes][Version:2][PkScript:var]
//	SKA: [CoinType:1][ValLen:1][Value:N bytes][Version:2][PkScript:var]
func writeTxOutV13(w io.Writer, pver uint32, version uint16, to *TxOut) error {
	// Write CoinType first
	err := binarySerializer.PutUint8(w, uint8(to.CoinType))
	if err != nil {
		return err
	}

	// Write value based on coin type
	if to.CoinType.IsSKA() {
		// SKA: variable-length big.Int with 1-byte length prefix
		// SKA outputs are required to use SKAValue (validated by CheckTransactionSanity)
		var valueBytes []byte
		if to.SKAValue != nil && to.SKAValue.Sign() > 0 {
			valueBytes = to.SKAValue.Bytes()
		}
		// valueBytes is nil/empty for zero value (e.g., OP_RETURN outputs)

		if len(valueBytes) == 0 {
			// Zero value: just write length 0
			err = binarySerializer.PutUint8(w, 0)
			if err != nil {
				return err
			}
		} else {
			// Write length prefix + big-endian bytes
			if len(valueBytes) > 255 {
				return messageError("writeTxOutV13", ErrVarBytesTooLong,
					"SKA value exceeds maximum length of 255 bytes")
			}
			err = binarySerializer.PutUint8(w, uint8(len(valueBytes)))
			if err != nil {
				return err
			}
			_, err = w.Write(valueBytes)
			if err != nil {
				return err
			}
		}
	} else {
		// VAR: fixed 8-byte int64 little-endian
		err = binarySerializer.PutUint64(w, littleEndian, uint64(to.Value))
		if err != nil {
			return err
		}
	}

	// Version
	err = binarySerializer.PutUint16(w, littleEndian, to.Version)
	if err != nil {
		return err
	}

	// PkScript
	return WriteVarBytes(w, pver, to.PkScript)
}

// IsSKAEmissionTransaction returns whether the given transaction is an SKA
// emission transaction based on its structure. SKA emission transactions have
// null inputs with authorized signature scripts and only produce SKA outputs.
//
// This function performs fast detection for categorization purposes and is
// optimized for performance as it may be called frequently. For full validation
// including cryptographic signature verification, use
// ValidateAuthorizedSKAEmissionTransaction in the blockchain package.
//
// The signature script must contain: [SKA_marker:4][auth_version:1][nonce:8]
// [coin_type:1][amount:8][height:8][pubkey:33][sig_len:1][signature:var]
func IsSKAEmissionTransaction(tx *MsgTx) bool {
	// Fast path: basic structure validation (most common rejection)
	if len(tx.TxIn) != 1 || len(tx.TxOut) == 0 {
		return false
	}

	// Check null input (fast rejection for regular transactions)
	prevOut := tx.TxIn[0].PreviousOutPoint
	if !prevOut.Hash.IsEqual(&chainhash.Hash{}) || prevOut.Index != 0xffffffff {
		return false
	}

	// Check signature script has minimum length for SKA marker
	// Minimum for basic detection: 4 bytes for [0x01][S][K][A] marker
	// Full authorization requires 64+ bytes but that's validated elsewhere
	sigScript := tx.TxIn[0].SignatureScript
	if len(sigScript) < 4 {
		return false
	}

	// Check authorized format: [0x01][S][K][A]...
	if !(sigScript[0] == 0x01 && sigScript[1] == 0x53 &&
		sigScript[2] == 0x4b && sigScript[3] == 0x41) {
		return false
	}

	// Check all outputs are SKA coin types using standard method
	for _, txOut := range tx.TxOut {
		if !txOut.CoinType.IsSKA() {
			return false
		}
	}

	return true
}
