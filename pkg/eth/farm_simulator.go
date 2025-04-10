package eth

import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"time"
)

// FarmSimulator simulates farm performance for benchmarking
type FarmSimulator struct {
	// Base APY for each farm (10% with some variation)
	baseAPY map[int64]float64
	// Volatility factor for each farm
	volatility map[int64]float64
	// Last simulated performance
	lastPerformance map[int64]float64
	// Random source
	rng *rand.Rand
}

// NewFarmSimulator creates a new farm simulator
func NewFarmSimulator() *FarmSimulator {
	// Create a deterministic but time-seeded random source
	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	
	// Initialize base APY for each farm (1-8)
	baseAPY := make(map[int64]float64)
	volatility := make(map[int64]float64)
	lastPerformance := make(map[int64]float64)
	
	for i := int64(1); i <= 8; i++ {
		// Base APY around 10% with some variation per farm
		baseAPY[i] = 0.10 + (rng.Float64()*0.02 - 0.01) // 9-11%
		
		// Different volatility for different farms
		// Higher farm IDs have higher volatility (more aggressive strategies)
		volatility[i] = 0.01 + (float64(i) * 0.005) // 1.5% to 5% volatility
		
		// Initialize with base performance
		lastPerformance[i] = baseAPY[i]
	}
	
	return &FarmSimulator{
		baseAPY:         baseAPY,
		volatility:      volatility,
		lastPerformance: lastPerformance,
		rng:             rng,
	}
}

// SimulateFarmPerformance simulates the current performance of a farm
// Returns the APY as a percentage (e.g., 10.5 for 10.5%)
func (fs *FarmSimulator) SimulateFarmPerformance(farmID int64) float64 {
	// Check if farm ID is valid
	if farmID < 1 || farmID > 8 {
		return 0
	}
	
	// Get base APY and volatility
	baseAPY := fs.baseAPY[farmID]
	volatility := fs.volatility[farmID]
	lastPerf := fs.lastPerformance[farmID]
	
	// Simulate market movement (random walk with mean reversion)
	// 70% weight to base APY, 20% to last performance, 10% random
	newPerf := (baseAPY * 0.7) + (lastPerf * 0.2) + ((fs.rng.Float64()*2-1) * volatility)
	
	// Ensure performance stays within reasonable bounds
	// Min: 20% below base, Max: 30% above base
	minAPY := baseAPY * 0.8
	maxAPY := baseAPY * 1.3
	
	if newPerf < minAPY {
		newPerf = minAPY
	} else if newPerf > maxAPY {
		newPerf = maxAPY
	}
	
	// Update last performance
	fs.lastPerformance[farmID] = newPerf
	
	// Return as percentage
	return newPerf * 100
}

// GetFarmScore calculates a score for the farm based on its performance
// Returns a score in basis points (0-10000, where 10000 = 100%)
func (fs *FarmSimulator) GetFarmScore(farmID int64) int64 {
	// Simulate performance
	performance := fs.SimulateFarmPerformance(farmID)
	
	// Calculate score based on performance
	// Base: 5000 (50%)
	// Each 1% of APY adds 500 points
	// So 10% APY would score 5000 + (10 * 500) = 10000 (100%)
	score := 5000 + int64(performance*500)
	
	// Cap at 10000 (100%)
	if score > 10000 {
		score = 10000
	} else if score < 0 {
		score = 0
	}
	
	return score
}

// GetRecommendedBenchmark calculates a recommended benchmark (target APY)
// based on the farm's recent performance
// Returns a benchmark in basis points (0-10000, where 10000 = 100%)
func (fs *FarmSimulator) GetRecommendedBenchmark(farmID int64) int64 {
	// Get current performance
	currentPerf := fs.SimulateFarmPerformance(farmID)
	
	// Recommended benchmark is slightly above current performance
	// Add 0.5% to current APY as a target
	recommendedAPY := currentPerf + 0.5
	
	// Convert to basis points (1% = 100 basis points)
	benchmark := int64(recommendedAPY * 100)
	
	// Cap at 10000 (100%)
	if benchmark > 10000 {
		benchmark = 10000
	}
	
	return benchmark
}

// GetAllFarmPerformance returns the performance of all farms
func (fs *FarmSimulator) GetAllFarmPerformance() map[int64]float64 {
	result := make(map[int64]float64)
	
	for i := int64(1); i <= 8; i++ {
		result[i] = fs.SimulateFarmPerformance(i)
	}
	
	return result
}

// FormatBasisPoints formats basis points as a percentage string
func FormatBasisPoints(basisPoints int64) string {
	percentage := float64(basisPoints) / 100.0
	return fmt.Sprintf("%.2f%%", percentage)
}

// ConvertAPYToBasisPoints converts an APY percentage to basis points
func ConvertAPYToBasisPoints(apy float64) int64 {
	return int64(math.Round(apy * 100))
}

// ConvertBasisPointsToAPY converts basis points to an APY percentage
func ConvertBasisPointsToAPY(basisPoints int64) float64 {
	return float64(basisPoints) / 100.0
}

// ConvertBasisPointsToBigInt converts basis points to a big.Int
func ConvertBasisPointsToBigInt(basisPoints int64) *big.Int {
	return big.NewInt(basisPoints)
}
