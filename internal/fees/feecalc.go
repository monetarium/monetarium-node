// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package fees

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/decred/dcrd/chaincfg/v3"
	"github.com/decred/dcrd/dcrutil/v4"
	"github.com/decred/dcrd/wire"
)

// CoinTypeFeeRate represents fee rate configuration for a specific coin type
type CoinTypeFeeRate struct {
	// MinRelayFee is the minimum fee rate for relay (atoms per KB)
	MinRelayFee dcrutil.Amount

	// DynamicFeeMultiplier adjusts fees based on network utilization
	DynamicFeeMultiplier float64

	// MaxFeeRate is the maximum allowed fee rate to prevent abuse
	MaxFeeRate dcrutil.Amount

	// LastUpdated tracks when this fee rate was last calculated
	LastUpdated time.Time
}

// CoinTypeFeeCalculator manages fee calculation and estimation for all coin types
type CoinTypeFeeCalculator struct {
	mu sync.RWMutex

	// chainParams contains network-specific parameters
	chainParams *chaincfg.Params

	// feeRates maps coin types to their current fee rates
	feeRates map[wire.CoinType]*CoinTypeFeeRate

	// utilizationStats tracks network utilization per coin type
	utilizationStats map[wire.CoinType]*UtilizationStats

	// defaultMinRelayFee is the baseline minimum relay fee
	defaultMinRelayFee dcrutil.Amount

	// updateInterval controls how often fee rates are recalculated
	updateInterval time.Duration
}

// UtilizationStats tracks network utilization metrics for dynamic fee calculation
type UtilizationStats struct {
	// PendingTxCount is the current number of pending transactions
	PendingTxCount int

	// PendingTxSize is the total size of pending transactions
	PendingTxSize int64

	// BlockSpaceUsed is the percentage of allocated block space being used
	BlockSpaceUsed float64

	// AvgConfirmationTime is the average time to confirmation
	AvgConfirmationTime time.Duration

	// RecentTxFees tracks recent transaction fees for this coin type
	RecentTxFees []int64

	// LastBlockIncluded tracks when transactions were last included in blocks
	LastBlockIncluded time.Time
}

// NewCoinTypeFeeCalculator creates a new fee calculator for the dual-coin system
func NewCoinTypeFeeCalculator(chainParams *chaincfg.Params, defaultMinRelayFee dcrutil.Amount) *CoinTypeFeeCalculator {
	calc := &CoinTypeFeeCalculator{
		chainParams:        chainParams,
		feeRates:           make(map[wire.CoinType]*CoinTypeFeeRate),
		utilizationStats:   make(map[wire.CoinType]*UtilizationStats),
		defaultMinRelayFee: defaultMinRelayFee,
		updateInterval:     time.Minute * 5, // Update every 5 minutes
	}

	// Initialize default fee rates for VAR and SKA
	calc.initializeDefaultFeeRates()

	return calc
}

// initializeDefaultFeeRates sets up initial fee rates for supported coin types
func (calc *CoinTypeFeeCalculator) initializeDefaultFeeRates() {
	now := time.Now()

	// VAR (Varta) coin fee rates
	calc.feeRates[wire.CoinTypeVAR] = &CoinTypeFeeRate{
		MinRelayFee:          calc.defaultMinRelayFee,
		DynamicFeeMultiplier: 1.0,
		MaxFeeRate:           calc.defaultMinRelayFee * 100, // 100x max
		LastUpdated:          now,
	}

	// SKA (Skarb) coin fee rates - start with lower base fee
	skaMinFee := calc.defaultMinRelayFee / 10 // 10x lower than VAR
	if calc.chainParams.SKAMinRelayTxFee > 0 {
		skaMinFee = dcrutil.Amount(calc.chainParams.SKAMinRelayTxFee)
	}

	calc.feeRates[wire.CoinTypeSKA] = &CoinTypeFeeRate{
		MinRelayFee:          skaMinFee,
		DynamicFeeMultiplier: 1.0,
		MaxFeeRate:           skaMinFee * 100, // 100x max
		LastUpdated:          now,
	}

	// Initialize utilization stats
	calc.utilizationStats[wire.CoinTypeVAR] = &UtilizationStats{
		RecentTxFees:      make([]int64, 0, 100),
		LastBlockIncluded: now,
	}

	calc.utilizationStats[wire.CoinTypeSKA] = &UtilizationStats{
		RecentTxFees:      make([]int64, 0, 100),
		LastBlockIncluded: now,
	}
}

// CalculateMinFee calculates the minimum fee for a transaction of the given size and coin type
func (calc *CoinTypeFeeCalculator) CalculateMinFee(serializedSize int64, coinType wire.CoinType) int64 {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		// Default to VAR fee calculation for unknown coin types
		feeRate = calc.feeRates[wire.CoinTypeVAR]
	}

	// Base fee calculation: (size in bytes * fee rate per KB) / 1000
	baseFee := (serializedSize * int64(feeRate.MinRelayFee)) / 1000

	// Apply dynamic multiplier based on network utilization
	dynamicFee := float64(baseFee) * feeRate.DynamicFeeMultiplier

	// Ensure minimum fee is at least 1 atom if fee rate > 0
	if dynamicFee == 0 && feeRate.MinRelayFee > 0 {
		dynamicFee = float64(feeRate.MinRelayFee)
	}

	// Enforce maximum fee limit
	maxFee := (serializedSize * int64(feeRate.MaxFeeRate)) / 1000
	if dynamicFee > float64(maxFee) {
		dynamicFee = float64(maxFee)
	}

	// Ensure fee is within valid monetary range
	finalFee := int64(dynamicFee)
	if finalFee < 0 || finalFee > dcrutil.MaxAmount {
		finalFee = dcrutil.MaxAmount
	}

	return finalFee
}

// EstimateFeeRate returns the current fee rate estimate for the given coin type and target confirmation blocks
func (calc *CoinTypeFeeCalculator) EstimateFeeRate(coinType wire.CoinType, targetConfirmations int) (dcrutil.Amount, error) {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		return 0, fmt.Errorf("unsupported coin type: %d", coinType)
	}

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		return feeRate.MinRelayFee, nil
	}

	// Calculate estimated fee rate based on recent transactions and target confirmations
	estimatedRate := feeRate.MinRelayFee

	// Factor in dynamic multiplier
	estimatedRate = dcrutil.Amount(float64(estimatedRate) * feeRate.DynamicFeeMultiplier)

	// Adjust based on target confirmations - faster confirmation = higher fee
	confirmationMultiplier := calc.calculateConfirmationMultiplier(targetConfirmations, stats)
	estimatedRate = dcrutil.Amount(float64(estimatedRate) * confirmationMultiplier)

	// Ensure within bounds
	if estimatedRate > feeRate.MaxFeeRate {
		estimatedRate = feeRate.MaxFeeRate
	}
	if estimatedRate < feeRate.MinRelayFee {
		estimatedRate = feeRate.MinRelayFee
	}

	return estimatedRate, nil
}

// calculateConfirmationMultiplier determines fee multiplier based on target confirmations
func (calc *CoinTypeFeeCalculator) calculateConfirmationMultiplier(targetConfirmations int, stats *UtilizationStats) float64 {
	// Base multiplier
	multiplier := 1.0

	// Faster confirmation requires higher fees
	if targetConfirmations <= 1 {
		multiplier = 2.0 // 2x for next block
	} else if targetConfirmations <= 3 {
		multiplier = 1.5 // 1.5x for fast confirmation
	} else if targetConfirmations <= 6 {
		multiplier = 1.2 // 1.2x for normal confirmation
	}

	// Adjust based on current utilization
	if stats.BlockSpaceUsed > 0.8 { // >80% utilization
		multiplier *= 1.5
	} else if stats.BlockSpaceUsed > 0.6 { // >60% utilization
		multiplier *= 1.2
	}

	return multiplier
}

// UpdateUtilization updates network utilization stats for dynamic fee calculation
func (calc *CoinTypeFeeCalculator) UpdateUtilization(coinType wire.CoinType, pendingTxCount int,
	pendingTxSize int64, blockSpaceUsed float64) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		stats = &UtilizationStats{
			RecentTxFees: make([]int64, 0, 100),
		}
		calc.utilizationStats[coinType] = stats
	}

	stats.PendingTxCount = pendingTxCount
	stats.PendingTxSize = pendingTxSize
	stats.BlockSpaceUsed = blockSpaceUsed

	// Update dynamic fee multiplier based on utilization
	calc.updateDynamicFeeMultiplier(coinType, stats)
}

// updateDynamicFeeMultiplier adjusts fee multiplier based on network conditions
func (calc *CoinTypeFeeCalculator) updateDynamicFeeMultiplier(coinType wire.CoinType, stats *UtilizationStats) {
	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		return
	}

	// Calculate new multiplier based on utilization
	newMultiplier := 1.0

	// Factor 1: Block space utilization
	if stats.BlockSpaceUsed > 0.9 {
		newMultiplier *= 2.0 // 2x when >90% utilized
	} else if stats.BlockSpaceUsed > 0.7 {
		newMultiplier *= 1.5 // 1.5x when >70% utilized
	} else if stats.BlockSpaceUsed > 0.5 {
		newMultiplier *= 1.2 // 1.2x when >50% utilized
	}

	// Factor 2: Pending transaction backlog
	if stats.PendingTxCount > 100 {
		newMultiplier *= 1.5
	} else if stats.PendingTxCount > 50 {
		newMultiplier *= 1.2
	}

	// Factor 3: Time since last block inclusion
	timeSinceLastBlock := time.Since(stats.LastBlockIncluded)
	if timeSinceLastBlock > time.Minute*10 {
		newMultiplier *= 1.3 // Boost fees if no recent confirmations
	}

	// Smooth the transition (weighted average)
	const smoothingFactor = 0.3
	feeRate.DynamicFeeMultiplier = (1-smoothingFactor)*feeRate.DynamicFeeMultiplier +
		smoothingFactor*newMultiplier

	// Enforce bounds
	if feeRate.DynamicFeeMultiplier > 10.0 {
		feeRate.DynamicFeeMultiplier = 10.0 // Max 10x multiplier
	}
	if feeRate.DynamicFeeMultiplier < 0.5 {
		feeRate.DynamicFeeMultiplier = 0.5 // Min 0.5x multiplier
	}

	feeRate.LastUpdated = time.Now()
}

// RecordTransactionFee records a transaction fee for fee estimation
func (calc *CoinTypeFeeCalculator) RecordTransactionFee(coinType wire.CoinType, fee int64, size int64, confirmed bool) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		stats = &UtilizationStats{
			RecentTxFees: make([]int64, 0, 100),
		}
		calc.utilizationStats[coinType] = stats
	}

	// Calculate fee rate (atoms per KB)
	feeRate := (fee * 1000) / size

	// Add to recent fees (keep last 100)
	stats.RecentTxFees = append(stats.RecentTxFees, feeRate)
	if len(stats.RecentTxFees) > 100 {
		stats.RecentTxFees = stats.RecentTxFees[1:]
	}

	// Update last block inclusion time if confirmed
	if confirmed {
		stats.LastBlockIncluded = time.Now()
	}
}

// GetFeeStats returns current fee statistics for a coin type
func (calc *CoinTypeFeeCalculator) GetFeeStats(coinType wire.CoinType) (*CoinTypeFeeStats, error) {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		return nil, fmt.Errorf("unsupported coin type: %d", coinType)
	}

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		return &CoinTypeFeeStats{
			CoinType:             coinType,
			MinRelayFee:          feeRate.MinRelayFee,
			DynamicFeeMultiplier: feeRate.DynamicFeeMultiplier,
			MaxFeeRate:           feeRate.MaxFeeRate,
		}, nil
	}

	// Calculate percentile fees from recent transactions
	percentileFees := calc.calculatePercentileFees(stats.RecentTxFees)

	return &CoinTypeFeeStats{
		CoinType:             coinType,
		MinRelayFee:          feeRate.MinRelayFee,
		DynamicFeeMultiplier: feeRate.DynamicFeeMultiplier,
		MaxFeeRate:           feeRate.MaxFeeRate,
		PendingTxCount:       stats.PendingTxCount,
		PendingTxSize:        stats.PendingTxSize,
		BlockSpaceUsed:       stats.BlockSpaceUsed,
		FastFee:              percentileFees[0], // 90th percentile
		NormalFee:            percentileFees[1], // 50th percentile
		SlowFee:              percentileFees[2], // 10th percentile
		LastUpdated:          feeRate.LastUpdated,
	}, nil
}

// CoinTypeFeeStats contains fee statistics for a specific coin type
type CoinTypeFeeStats struct {
	CoinType             wire.CoinType  `json:"cointype"`
	MinRelayFee          dcrutil.Amount `json:"minrelayfee"`
	DynamicFeeMultiplier float64        `json:"dynamicfeemultiplier"`
	MaxFeeRate           dcrutil.Amount `json:"maxfeerate"`
	PendingTxCount       int            `json:"pendingtxcount"`
	PendingTxSize        int64          `json:"pendingtxsize"`
	BlockSpaceUsed       float64        `json:"blockspaceused"`
	FastFee              dcrutil.Amount `json:"fastfee"`   // ~1 block (90th percentile)
	NormalFee            dcrutil.Amount `json:"normalfee"` // ~3 blocks (50th percentile)
	SlowFee              dcrutil.Amount `json:"slowfee"`   // ~6 blocks (10th percentile)
	LastUpdated          time.Time      `json:"lastupdated"`
}

// calculatePercentileFees calculates fee percentiles from recent transaction data
func (calc *CoinTypeFeeCalculator) calculatePercentileFees(recentFees []int64) [3]dcrutil.Amount {
	if len(recentFees) == 0 {
		// Return default fees if no data
		return [3]dcrutil.Amount{
			calc.defaultMinRelayFee * 2, // Fast
			calc.defaultMinRelayFee,     // Normal
			calc.defaultMinRelayFee / 2, // Slow
		}
	}

	// Sort fees for percentile calculation
	sortedFees := make([]int64, len(recentFees))
	copy(sortedFees, recentFees)

	// Simple insertion sort for small arrays
	for i := 1; i < len(sortedFees); i++ {
		key := sortedFees[i]
		j := i - 1
		for j >= 0 && sortedFees[j] > key {
			sortedFees[j+1] = sortedFees[j]
			j--
		}
		sortedFees[j+1] = key
	}

	// Calculate percentiles
	p90 := calcPercentile(sortedFees, 0.90) // Fast fee
	p50 := calcPercentile(sortedFees, 0.50) // Normal fee
	p10 := calcPercentile(sortedFees, 0.10) // Slow fee

	return [3]dcrutil.Amount{
		dcrutil.Amount(p90),
		dcrutil.Amount(p50),
		dcrutil.Amount(p10),
	}
}

// calcPercentile calculates the given percentile from sorted data
func calcPercentile(sortedData []int64, percentile float64) int64 {
	if len(sortedData) == 0 {
		return 0
	}

	index := percentile * float64(len(sortedData)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sortedData[lower]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return int64(float64(sortedData[lower])*(1-weight) + float64(sortedData[upper])*weight)
}

// ValidateTransactionFees validates fees for a transaction, ensuring they meet coin-type-specific requirements
func (calc *CoinTypeFeeCalculator) ValidateTransactionFees(txFee int64, serializedSize int64,
	coinType wire.CoinType, allowHighFees bool) error {

	// Calculate minimum required fee
	minFee := calc.CalculateMinFee(serializedSize, coinType)

	if txFee < minFee {
		return fmt.Errorf("insufficient fee for coin type %d: %d < %d atoms",
			coinType, txFee, minFee)
	}

	// Check maximum fee if not allowing high fees
	if !allowHighFees {
		calc.mu.RLock()
		feeRate, exists := calc.feeRates[coinType]
		calc.mu.RUnlock()

		if exists {
			maxFee := (serializedSize * int64(feeRate.MaxFeeRate)) / 1000
			if txFee > maxFee {
				return fmt.Errorf("fee too high for coin type %d: %d > %d atoms",
					coinType, txFee, maxFee)
			}
		}
	}

	return nil
}

// GetSupportedCoinTypes returns a list of coin types supported by the fee calculator
func (calc *CoinTypeFeeCalculator) GetSupportedCoinTypes() []wire.CoinType {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	coinTypes := make([]wire.CoinType, 0, len(calc.feeRates))
	for coinType := range calc.feeRates {
		coinTypes = append(coinTypes, coinType)
	}

	return coinTypes
}
