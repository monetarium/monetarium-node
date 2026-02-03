// Copyright (c) 2016 The btcsuite developers
// Copyright (c) 2015-2022 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"container/heap"
	"math/big"
	"math/rand"
	"testing"

	"github.com/monetarium/monetarium-node/blockchain/stake"
)

// TestStakeTxFeePrioHeap tests the priority heap including the stake types for
// both transaction fees per KB and transaction priority.  It ensures that the
// primary sorting is first by stake type, and then by the priority type.
func TestStakeTxFeePrioHeap(t *testing.T) {
	numTestItems := 1000

	// Create some fake priority items that exercise the expected sort
	// edge conditions.
	testItems := []*txPrioItem{
		{feePerKB: big.NewInt(5678), txType: stake.TxTypeRegular, priority: 3},
		{feePerKB: big.NewInt(5678), txType: stake.TxTypeRegular, priority: 1},
		{feePerKB: big.NewInt(5678), txType: stake.TxTypeRegular, priority: 1}, // Duplicate fee and prio
		{feePerKB: big.NewInt(5678), txType: stake.TxTypeRegular, priority: 5},
		{feePerKB: big.NewInt(5678), txType: stake.TxTypeRegular, priority: 2},
		{feePerKB: big.NewInt(1234), txType: stake.TxTypeRegular, priority: 3},
		{feePerKB: big.NewInt(1234), txType: stake.TxTypeRegular, priority: 1},
		{feePerKB: big.NewInt(1234), txType: stake.TxTypeRegular, priority: 5},
		{feePerKB: big.NewInt(1234), txType: stake.TxTypeRegular, priority: 5}, // Duplicate fee and prio
		{feePerKB: big.NewInt(1234), txType: stake.TxTypeRegular, priority: 2},
		{feePerKB: big.NewInt(10000), txType: stake.TxTypeRegular, priority: 0}, // Higher fee, lower prio
		{feePerKB: big.NewInt(0), txType: stake.TxTypeRegular, priority: 10000}, // Higher prio, lower fee
		{feePerKB: big.NewInt(0), txType: stake.TxTypeSSRtx, autoRevocation: true},
	}

	// Add random data in addition to the edge conditions already manually
	// specified.
	for i := len(testItems); i < numTestItems; i++ {
		randType := stake.TxType(rand.Intn(4))
		randPrio := rand.Float64() * 100
		randFeePerKB := int64(rand.Float64() * 10000) // Use int64 for random fee
		testItems = append(testItems, &txPrioItem{
			txDesc:   nil,
			txType:   randType,
			feePerKB: big.NewInt(randFeePerKB),
			priority: randPrio,
		})
	}

	// Test sorting by stake and fee per KB.
	ph := newTxPriorityQueue(numTestItems, txPQByStakeAndFee)
	for i := 0; i < numTestItems; i++ {
		heap.Push(ph, testItems[i])
	}
	last := &txPrioItem{
		txDesc:   nil,
		txType:   stake.TxTypeSSGen,
		priority: 10000.0,
		feePerKB: big.NewInt(10000),
	}
	for i := 0; i < numTestItems; i++ {
		prioItem := heap.Pop(ph)
		txpi, ok := prioItem.(*txPrioItem)
		if ok {
			if txpi.feePerKB.Cmp(last.feePerKB) > 0 &&
				compareStakePriority(txpi, last) >= 0 {
				t.Errorf("bad pop: %v fee per KB was more than last of %v "+
					"while the txtype was %v but last was %v",
					txpi.feePerKB, last.feePerKB, txpi.txType, last.txType)
			}
			last = txpi
		}
	}
}
