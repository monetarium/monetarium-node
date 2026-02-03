// Copyright (c) 2025 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package fees

import (
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/cointype"
	"github.com/monetarium/monetarium-node/dcrutil"
)

// CoinTypeFeeRate represents fee rate configuration for a specific coin type
type CoinTypeFeeRate struct {
	// MinRelayFee is the minimum fee rate for relay (atoms per KB)
	// Used for VAR transactions
	MinRelayFee dcrutil.Amount

	// DynamicFeeMultiplier adjusts fees based on network utilization
	DynamicFeeMultiplier float64

	// MaxFeeRate is the maximum allowed fee rate to prevent abuse
	// Used for VAR transactions (100x min fee)
	MaxFeeRate dcrutil.Amount

	// SKAMinRelayFee is the minimum fee rate for SKA transactions (atoms per KB)
	// Uses big.Int to support SKA's 18 decimal precision
	SKAMinRelayFee *big.Int

	// SKAMaxFeeRate is the maximum allowed fee rate for SKA transactions
	// Set to total emission (MaxSupply) as the logical maximum
	SKAMaxFeeRate *big.Int

	// LastUpdated tracks when this fee rate was last calculated
	LastUpdated time.Time
}

// CoinTypeFeeCalculator manages fee calculation and estimation for all coin types
type CoinTypeFeeCalculator struct {
	mu sync.RWMutex

	// chainParams contains network-specific parameters
	chainParams *chaincfg.Params

	// feeRates maps coin types to their current fee rates
	feeRates map[cointype.CoinType]*CoinTypeFeeRate

	// utilizationStats tracks network utilization per coin type
	utilizationStats map[cointype.CoinType]*UtilizationStats

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

	// RecentTxFees tracks recent transaction fee rates (atoms/KB) for this coin type.
	// Uses *big.Int to support both VAR (int64 range) and SKA (bigint).
	RecentTxFees []*big.Int

	// LastBlockIncluded tracks when transactions were last included in blocks
	LastBlockIncluded time.Time
}

// NewCoinTypeFeeCalculator creates a new fee calculator for the dual-coin system
func NewCoinTypeFeeCalculator(chainParams *chaincfg.Params, defaultMinRelayFee dcrutil.Amount) *CoinTypeFeeCalculator {
	calc := &CoinTypeFeeCalculator{
		chainParams:        chainParams,
		feeRates:           make(map[cointype.CoinType]*CoinTypeFeeRate),
		utilizationStats:   make(map[cointype.CoinType]*UtilizationStats),
		defaultMinRelayFee: defaultMinRelayFee,
		updateInterval:     time.Minute * 5, // Update every 5 minutes
	}

	// Initialize default fee rates for VAR and SKA
	calc.initializeDefaultFeeRates()

	return calc
}

// initializeDefaultFeeRates sets up the initial fee rates for VAR.
// SKA coin types are initialized on-demand using helper methods.
func (calc *CoinTypeFeeCalculator) initializeDefaultFeeRates() {
	now := time.Now()

	// VAR (Varta) coin fee rates - VAR has unique properties and is always active
	calc.feeRates[cointype.CoinTypeVAR] = &CoinTypeFeeRate{
		MinRelayFee:          calc.defaultMinRelayFee,
		DynamicFeeMultiplier: 1.0,
		MaxFeeRate:           calc.defaultMinRelayFee * 100, // 100x max
		LastUpdated:          now,
	}

	// Initialize VAR utilization stats
	calc.utilizationStats[cointype.CoinTypeVAR] = &UtilizationStats{
		RecentTxFees:      make([]*big.Int, 0, 100),
		LastBlockIncluded: now,
	}

	// Initialize all active SKA coins from chain configuration
	for coinType, config := range calc.chainParams.SKACoins {
		if config.Active {
			calc.feeRates[coinType] = calc.getDefaultSKAFeeRate(coinType, config)
			calc.utilizationStats[coinType] = &UtilizationStats{
				RecentTxFees:      make([]*big.Int, 0, 100),
				LastBlockIncluded: now,
			}
		}
	}
}

// getDefaultSKAFeeRate returns default fee rate configuration for SKA coin types.
// This method reads fee configuration from the per-coin SKACoinConfig.
// The maxFeeRate is set to MaxFeeMultiplier * MinRelayTxFee (default 2500x).
func (calc *CoinTypeFeeCalculator) getDefaultSKAFeeRate(coinType cointype.CoinType, config *chaincfg.SKACoinConfig) *CoinTypeFeeRate {
	// Get fee rate from coin config (uses *big.Int to support > 9.22 SKA)
	skaMinFeeBig := config.MinRelayTxFee
	if skaMinFeeBig == nil || skaMinFeeBig.Sign() <= 0 {
		// Fallback to default: 4 SKA per KB
		skaMinFeeBig = big.NewInt(4000000000000000000)
	}

	// Calculate max fee as multiplier * min fee (default 2500x)
	multiplier := config.MaxFeeMultiplier
	if multiplier <= 0 {
		multiplier = 2500 // Default multiplier
	}
	skaMaxFee := new(big.Int).Mul(skaMinFeeBig, big.NewInt(multiplier))

	// Cap at MaxSupply as absolute maximum
	if skaMaxFee.Cmp(config.MaxSupply) > 0 {
		skaMaxFee = new(big.Int).Set(config.MaxSupply)
	}

	return &CoinTypeFeeRate{
		MinRelayFee:          0, // Not used for SKA - use SKAMinRelayFee instead
		DynamicFeeMultiplier: 1.0,
		MaxFeeRate:           0, // Not used for SKA - use SKAMaxFeeRate instead
		SKAMinRelayFee:       skaMinFeeBig,
		SKAMaxFeeRate:        skaMaxFee,
		LastUpdated:          time.Now(),
	}
}

// CalculateMinFee calculates the minimum fee for a transaction of the given size and coin type
// CalculateMinFee calculates the minimum fee for a transaction of the given size and coin type.
// Returns *big.Int to support both VAR (int64 range) and SKA (18 decimal precision).
// For VAR, callers can safely use .Int64() on the result.
func (calc *CoinTypeFeeCalculator) CalculateMinFee(serializedSize int64, coinType cointype.CoinType) *big.Int {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		// Default to VAR fee calculation for unknown coin types
		feeRate = calc.feeRates[cointype.CoinTypeVAR]
	}

	sizeBig := big.NewInt(serializedSize)

	// SKA path: use SKAMinRelayFee (big.Int)
	if coinType.IsSKA() && feeRate.SKAMinRelayFee != nil {
		// Base fee calculation: (size in bytes * fee rate per KB) / 1000
		baseFee := new(big.Int).Mul(sizeBig, feeRate.SKAMinRelayFee)
		baseFee.Div(baseFee, cointype.KilobyteInt)

		// Apply dynamic multiplier (convert to fixed-point arithmetic)
		if feeRate.DynamicFeeMultiplier != 1.0 {
			multiplierFixed := int64(feeRate.DynamicFeeMultiplier * 1000)
			baseFee.Mul(baseFee, big.NewInt(multiplierFixed))
			baseFee.Div(baseFee, cointype.KilobyteInt)
		}

		// Ensure minimum fee is at least the min relay fee if > 0
		if baseFee.Sign() == 0 && feeRate.SKAMinRelayFee.Sign() > 0 {
			baseFee = new(big.Int).Set(feeRate.SKAMinRelayFee)
		}

		return baseFee
	}

	// VAR path: use MinRelayFee (int64-based)
	// Base fee calculation: (size in bytes * fee rate per KB) / 1000
	baseFee := (serializedSize * int64(feeRate.MinRelayFee)) / 1000

	// Apply dynamic multiplier based on network utilization
	dynamicFee := float64(baseFee) * feeRate.DynamicFeeMultiplier

	// Ensure minimum fee is at least the min relay fee if > 0
	if dynamicFee == 0 && feeRate.MinRelayFee > 0 {
		dynamicFee = float64(feeRate.MinRelayFee)
	}

	// Enforce maximum fee limit for VAR
	maxFee := (serializedSize * int64(feeRate.MaxFeeRate)) / 1000
	if dynamicFee > float64(maxFee) && maxFee > 0 {
		dynamicFee = float64(maxFee)
	}

	// Ensure fee is within valid monetary range
	finalFee := int64(dynamicFee)
	if finalFee < 0 || finalFee > int64(cointype.MaxVARAmount) {
		finalFee = int64(cointype.MaxVARAmount)
	}

	return big.NewInt(finalFee)
}

// EstimateFeeRate returns the current fee rate estimate for the given coin type
// and target confirmation blocks. Returns *big.Int to support both VAR and SKA.
// For VAR, the result fits in int64 and can be safely converted with .Int64().
func (calc *CoinTypeFeeCalculator) EstimateFeeRate(coinType cointype.CoinType, targetConfirmations int) (*big.Int, error) {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		return nil, fmt.Errorf("unsupported coin type: %d", coinType)
	}

	stats := calc.utilizationStats[coinType]

	// Get base fee rate as *big.Int
	var minFeeRate, maxFeeRate *big.Int
	if coinType.IsSKA() && feeRate.SKAMinRelayFee != nil {
		minFeeRate = feeRate.SKAMinRelayFee
		maxFeeRate = feeRate.SKAMaxFeeRate
	} else {
		minFeeRate = big.NewInt(int64(feeRate.MinRelayFee))
		maxFeeRate = big.NewInt(int64(feeRate.MaxFeeRate))
	}

	estimatedRate := new(big.Int).Set(minFeeRate)

	// Apply dynamic multiplier using fixed-point arithmetic (multiplier * 1000)
	if feeRate.DynamicFeeMultiplier != 1.0 {
		multiplierFixed := int64(feeRate.DynamicFeeMultiplier * 1000)
		estimatedRate.Mul(estimatedRate, big.NewInt(multiplierFixed))
		estimatedRate.Div(estimatedRate, big.NewInt(1000))
	}

	// Apply confirmation multiplier
	if stats != nil {
		confirmMultiplier := calc.calculateConfirmationMultiplier(targetConfirmations, stats)
		if confirmMultiplier != 1.0 {
			multiplierFixed := int64(confirmMultiplier * 1000)
			estimatedRate.Mul(estimatedRate, big.NewInt(multiplierFixed))
			estimatedRate.Div(estimatedRate, big.NewInt(1000))
		}
	}

	// Enforce bounds
	if maxFeeRate != nil && maxFeeRate.Sign() > 0 && estimatedRate.Cmp(maxFeeRate) > 0 {
		estimatedRate = new(big.Int).Set(maxFeeRate)
	}
	if estimatedRate.Cmp(minFeeRate) < 0 {
		estimatedRate = new(big.Int).Set(minFeeRate)
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
func (calc *CoinTypeFeeCalculator) UpdateUtilization(coinType cointype.CoinType, pendingTxCount int,
	pendingTxSize int64, blockSpaceUsed float64) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		stats = &UtilizationStats{
			RecentTxFees: make([]*big.Int, 0, 100),
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
func (calc *CoinTypeFeeCalculator) updateDynamicFeeMultiplier(coinType cointype.CoinType, stats *UtilizationStats) {
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

// RecordTransactionFee records a transaction fee for fee estimation.
// For VAR transactions, use this method with int64 fee.
// For SKA transactions with bigint fees, use RecordTransactionFeeBig.
func (calc *CoinTypeFeeCalculator) RecordTransactionFee(coinType cointype.CoinType, fee int64, size int64, confirmed bool) {
	calc.RecordTransactionFeeBig(coinType, big.NewInt(fee), size, confirmed)
}

// RecordTransactionFeeBig records a transaction fee for fee estimation using big.Int.
// This supports SKA transactions with fees that exceed int64.
func (calc *CoinTypeFeeCalculator) RecordTransactionFeeBig(coinType cointype.CoinType, fee *big.Int, size int64, confirmed bool) {
	calc.mu.Lock()
	defer calc.mu.Unlock()

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		stats = &UtilizationStats{
			RecentTxFees: make([]*big.Int, 0, 100),
		}
		calc.utilizationStats[coinType] = stats
	}

	// Calculate fee rate (atoms per KB): feeRate = (fee * 1000) / size
	if size <= 0 {
		return // Invalid size, skip recording
	}
	feeRate := new(big.Int).Mul(fee, cointype.KilobyteInt)
	feeRate.Div(feeRate, big.NewInt(size))

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

// GetFeeStats returns current fee statistics for a coin type.
// For SKA coins, the BigInt variants are also populated.
func (calc *CoinTypeFeeCalculator) GetFeeStats(coinType cointype.CoinType) (*CoinTypeFeeStats, error) {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	feeRate, exists := calc.feeRates[coinType]
	if !exists {
		return nil, fmt.Errorf("unsupported coin type: %d", coinType)
	}

	// Get fee values as *big.Int based on coin type
	var minRelayFee, maxFeeRate *big.Int
	if coinType.IsSKA() && feeRate.SKAMinRelayFee != nil {
		minRelayFee = new(big.Int).Set(feeRate.SKAMinRelayFee)
		maxFeeRate = new(big.Int).Set(feeRate.SKAMaxFeeRate)
	} else {
		minRelayFee = big.NewInt(int64(feeRate.MinRelayFee))
		maxFeeRate = big.NewInt(int64(feeRate.MaxFeeRate))
	}

	stats, exists := calc.utilizationStats[coinType]
	if !exists {
		return &CoinTypeFeeStats{
			CoinType:             coinType,
			MinRelayFee:          minRelayFee,
			DynamicFeeMultiplier: feeRate.DynamicFeeMultiplier,
			MaxFeeRate:           maxFeeRate,
		}, nil
	}

	// Calculate percentile fees from recent transactions using coin-type-specific min fee
	percentileFees := calc.calculatePercentileFees(stats.RecentTxFees, minRelayFee)

	return &CoinTypeFeeStats{
		CoinType:             coinType,
		MinRelayFee:          minRelayFee,
		DynamicFeeMultiplier: feeRate.DynamicFeeMultiplier,
		MaxFeeRate:           maxFeeRate,
		PendingTxCount:       stats.PendingTxCount,
		PendingTxSize:        stats.PendingTxSize,
		BlockSpaceUsed:       stats.BlockSpaceUsed,
		FastFee:              percentileFees[0], // 90th percentile
		NormalFee:            percentileFees[1], // 50th percentile
		SlowFee:              percentileFees[2], // 10th percentile
		LastUpdated:          feeRate.LastUpdated,
	}, nil
}

// CoinTypeFeeStats contains fee statistics for a specific coin type.
// All fee fields use *big.Int to support both VAR (int64 range) and SKA (bigint).
type CoinTypeFeeStats struct {
	CoinType             cointype.CoinType `json:"cointype"`
	MinRelayFee          *big.Int          `json:"minrelayfee"`
	DynamicFeeMultiplier float64           `json:"dynamicfeemultiplier"`
	MaxFeeRate           *big.Int          `json:"maxfeerate"`
	PendingTxCount       int               `json:"pendingtxcount"`
	PendingTxSize        int64             `json:"pendingtxsize"`
	BlockSpaceUsed       float64           `json:"blockspaceused"`
	FastFee              *big.Int          `json:"fastfee"`   // ~1 block (90th percentile)
	NormalFee            *big.Int          `json:"normalfee"` // ~3 blocks (50th percentile)
	SlowFee              *big.Int          `json:"slowfee"`   // ~6 blocks (10th percentile)
	LastUpdated          time.Time         `json:"lastupdated"`
}

// calculatePercentileFees calculates fee percentiles from recent transaction data.
// The minRelayFee parameter should be the coin-type-specific minimum relay fee,
// ensuring SKA coins use their own fee rate rather than VAR's default.
// All values use *big.Int to support both VAR and SKA.
func (calc *CoinTypeFeeCalculator) calculatePercentileFees(recentFees []*big.Int, minRelayFee *big.Int) [3]*big.Int {
	if len(recentFees) == 0 {
		// Return default fees based on coin-type-specific minRelayFee
		// All fees must be >= minRelayFee to be accepted by mempool
		return [3]*big.Int{
			new(big.Int).Mul(minRelayFee, big.NewInt(2)), // Fast: 2x min
			new(big.Int).Set(minRelayFee),                // Normal: 1x min
			new(big.Int).Set(minRelayFee),                // Slow: 1x min (can't go lower)
		}
	}

	// Sort fees for percentile calculation
	sortedFees := make([]*big.Int, len(recentFees))
	for i, f := range recentFees {
		sortedFees[i] = new(big.Int).Set(f)
	}

	// Simple insertion sort for small arrays (using big.Int comparison)
	for i := 1; i < len(sortedFees); i++ {
		key := sortedFees[i]
		j := i - 1
		for j >= 0 && sortedFees[j].Cmp(key) > 0 {
			sortedFees[j+1] = sortedFees[j]
			j--
		}
		sortedFees[j+1] = key
	}

	// Calculate percentiles
	p90 := calcPercentile(sortedFees, 0.90) // Fast fee
	p50 := calcPercentile(sortedFees, 0.50) // Normal fee
	p10 := calcPercentile(sortedFees, 0.10) // Slow fee

	// Enforce minimum relay fee floor using coin-type-specific minRelayFee
	// This ensures RPC fee estimates are always acceptable to the mempool
	if p90.Cmp(minRelayFee) < 0 {
		p90 = new(big.Int).Set(minRelayFee)
	}
	if p50.Cmp(minRelayFee) < 0 {
		p50 = new(big.Int).Set(minRelayFee)
	}
	if p10.Cmp(minRelayFee) < 0 {
		p10 = new(big.Int).Set(minRelayFee)
	}

	return [3]*big.Int{p90, p50, p10}
}

// calcPercentile calculates the given percentile from sorted *big.Int data
func calcPercentile(sortedData []*big.Int, percentile float64) *big.Int {
	if len(sortedData) == 0 {
		return big.NewInt(0)
	}

	index := percentile * float64(len(sortedData)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return new(big.Int).Set(sortedData[lower])
	}

	// Linear interpolation using fixed-point arithmetic
	// result = lower_val * (1 - weight) + upper_val * weight
	// The interpolation weight = index - floor(index), scaled by 1000 for precision
	weight := int64((index - float64(lower)) * 1000)

	lowerVal := new(big.Int).Mul(sortedData[lower], big.NewInt(1000-weight))
	upperVal := new(big.Int).Mul(sortedData[upper], big.NewInt(weight))
	result := new(big.Int).Add(lowerVal, upperVal)
	result.Div(result, big.NewInt(1000))

	return result
}

// ValidateTransactionFees validates fees for a transaction, ensuring they meet coin-type-specific requirements.
// txFee should be *big.Int to support both VAR and SKA.
func (calc *CoinTypeFeeCalculator) ValidateTransactionFees(txFee *big.Int, serializedSize int64,
	coinType cointype.CoinType, allowHighFees bool) error {

	if txFee == nil {
		return fmt.Errorf("nil fee for coin type %d", coinType)
	}

	// Calculate minimum required fee
	minFee := calc.CalculateMinFee(serializedSize, coinType)

	if txFee.Cmp(minFee) < 0 {
		return fmt.Errorf("insufficient fee for coin type %d: %s < %s atoms",
			coinType, txFee.String(), minFee.String())
	}

	// Check maximum fee if not allowing high fees
	if !allowHighFees {
		maxFee := calc.CalculateMaxFee(serializedSize, coinType)
		if maxFee != nil && txFee.Cmp(maxFee) > 0 {
			return fmt.Errorf("fee too high for coin type %d: %s > %s atoms",
				coinType, txFee.String(), maxFee.String())
		}
	}

	return nil
}

// CalculateMaxFee calculates the maximum allowed fee for a transaction.
// Returns *big.Int, or nil if no max fee limit is configured.
func (calc *CoinTypeFeeCalculator) CalculateMaxFee(serializedSize int64, coinType cointype.CoinType) *big.Int {
	calc.mu.RLock()
	feeRate, exists := calc.feeRates[coinType]
	calc.mu.RUnlock()

	if !exists {
		return nil
	}

	// SKA path: use SKAMaxFeeRate (big.Int)
	if coinType.IsSKA() && feeRate.SKAMaxFeeRate != nil {
		// Calculate max fee for this transaction size: maxFeeRate * size / 1000
		maxFeeForSize := new(big.Int).Mul(feeRate.SKAMaxFeeRate, big.NewInt(serializedSize))
		maxFeeForSize.Div(maxFeeForSize, big.NewInt(1000))

		// Ensure minimum of the max fee rate itself (for very small transactions)
		if maxFeeForSize.Cmp(feeRate.SKAMaxFeeRate) < 0 {
			maxFeeForSize = new(big.Int).Set(feeRate.SKAMaxFeeRate)
		}
		return maxFeeForSize
	}

	// VAR path: use MaxFeeRate (int64-based)
	if feeRate.MaxFeeRate == 0 {
		return nil
	}
	maxFee := (serializedSize * int64(feeRate.MaxFeeRate)) / 1000
	if maxFee < int64(feeRate.MaxFeeRate) {
		maxFee = int64(feeRate.MaxFeeRate)
	}
	return big.NewInt(maxFee)
}

// GetSupportedCoinTypes returns a list of coin types supported by the fee calculator
func (calc *CoinTypeFeeCalculator) GetSupportedCoinTypes() []cointype.CoinType {
	calc.mu.RLock()
	defer calc.mu.RUnlock()

	coinTypes := make([]cointype.CoinType, 0, len(calc.feeRates))
	for coinType := range calc.feeRates {
		coinTypes = append(coinTypes, coinType)
	}

	return coinTypes
}
