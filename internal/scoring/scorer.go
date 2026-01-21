package scoring

import (
	"fmt"
	"math"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/config"
)

type TokenScore struct {
	LiquidityScore float64
	VolumeScore    float64
	HolderScore    float64
	CompositeScore float64
	IsSafe         bool
	FailureReasons []string
}

func Scorer(
	isContractVerified bool,
	aggregatedLiquidityUSD float64,
	aggregatedVolume24hUSD float64,
	top10HoldersPercentage float64,
	isFragmentationSafe bool,
	largestSingleLiquidityPoolAgeDays float64,
) (TokenScore, bool) {
	cfg := config.Load()

	result := TokenScore{
		IsSafe:         true,
		FailureReasons: []string{},
	}

	// Hard Filters - Any failure = reject
	if !isContractVerified {
		result.IsSafe = false
		result.FailureReasons = append(result.FailureReasons, "Contract not verified")
	}

	if aggregatedLiquidityUSD < cfg.MinLiquidityUSD {
		result.IsSafe = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Liquidity too low: $%.2f < $%.2f", aggregatedLiquidityUSD, cfg.MinLiquidityUSD))
	}

	if aggregatedVolume24hUSD < cfg.MinVolume24h {
		result.IsSafe = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Volume too low: $%.2f < $%.2f", aggregatedVolume24hUSD, cfg.MinVolume24h))
	}

	if top10HoldersPercentage > cfg.MaxTop10HolderConcentration {
		result.IsSafe = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Holder concentration too high: %.2f%% > %.2f%%", top10HoldersPercentage, cfg.MaxTop10HolderConcentration))
	}

	if !isFragmentationSafe {
		result.IsSafe = false
		result.FailureReasons = append(result.FailureReasons, "Liquidity too fragmented")
	}

	if largestSingleLiquidityPoolAgeDays < 7.0 {
		result.IsSafe = false
		result.FailureReasons = append(result.FailureReasons,
			fmt.Sprintf("Pair too new: %.1f days < 7 days", largestSingleLiquidityPoolAgeDays))
	}

	// If any hard filter failed, return 0 score
	if !result.IsSafe {
		result.CompositeScore = 0
		return result, false
	}

	// Calculate individual scores (0-100)
	result.LiquidityScore = calculateLiquidityScore(aggregatedLiquidityUSD)
	result.VolumeScore = calculateVolumeScore(aggregatedVolume24hUSD, aggregatedLiquidityUSD)
	result.HolderScore = calculateHolderScore(top10HoldersPercentage)

	// Weighted composite score
	result.CompositeScore = (result.LiquidityScore*cfg.LiquidityWeight +
		result.VolumeScore*cfg.VolumeWeight +
		result.HolderScore*cfg.HolderWeight)

	return result, true
}

func calculateLiquidityScore(liquidityUSD float64) float64 {
	// $100k = 50, $1M = 100, $10M = 100
	score := math.Min(100, (liquidityUSD/1_000_000)*100)
	return score
}

func calculateVolumeScore(volume24h, liquidityUSD float64) float64 {
	if liquidityUSD == 0 {
		return 0
	}

	// Turnover ratio (volume/liquidity)
	turnover := volume24h / liquidityUSD

	// Ideal: 5-20% daily turnover
	if turnover >= 0.05 && turnover <= 0.20 {
		return 100
	} else if turnover > 0.20 {
		return 70 // High volatility
	} else {
		// Scale low turnover: 0-5% maps to 0-100
		return math.Min(100, turnover*2000)
	}
}

func calculateHolderScore(top10Percentage float64) float64 {
	// Lower concentration = better
	if top10Percentage < 30 {
		return 100
	} else if top10Percentage < 50 {
		return 80
	} else if top10Percentage < 70 {
		return 50
	} else {
		return 20
	}
}
