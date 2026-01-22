package scoring

import (
	"fmt"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/config"
)

type TokenScore struct {
	LiquidityScore     float64
	VolumeScore        float64
	HolderScore        float64
	FragmentationScore float64
	CompositeScore     float64
	IsSafe             bool
	FailureReasons     []string
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
	result.FragmentationScore = calculateFragmentationScore(isFragmentationSafe)

	// Weighted composite score
	result.CompositeScore = (result.LiquidityScore*cfg.LiquidityWeight +
		result.VolumeScore*cfg.VolumeWeight +
		result.HolderScore*cfg.HolderWeight +
		result.FragmentationScore*cfg.FragmentationWeight)

	return result, true
}

func calculateLiquidityScore(liquidityUSD float64) float64 {
	if liquidityUSD >= 5_000_000 {
		return 100
	} else if liquidityUSD >= 1_000_000 {
		return 80 + ((liquidityUSD-1_000_000)/4_000_000)*20 // 80-100
	} else if liquidityUSD >= 500_000 {
		return 60 + ((liquidityUSD-500_000)/500_000)*20 // 60-80
	} else if liquidityUSD >= 100_000 {
		return 30 + ((liquidityUSD-100_000)/400_000)*30 // 30-60
	} else {
		return 0
	}
}

func calculateVolumeScore(volume24h, liquidityUSD float64) float64 {
	if liquidityUSD == 0 {
		return 0
	}

	// For HIGH liquidity tokens, high turnover is GOOD
	if liquidityUSD >= 5_000_000 {
		// Major tokens: reward high activity
		if volume24h >= 50_000_000 { // $50M+ daily
			return 100
		} else if volume24h >= 10_000_000 { // $10M+
			return 95
		} else if volume24h >= 1_000_000 { // $1M+
			return 85
		} else {
			turnover := volume24h / liquidityUSD
			if turnover >= 0.10 { // 10%+
				return 80
			}
			return 60 // Low activity on major token
		}
	}

	// For MEDIUM liquidity ($500K-$5M): traditional turnover analysis
	turnover := volume24h / liquidityUSD
	if turnover >= 0.05 && turnover <= 0.30 { // 5-30%
		return 100
	} else if turnover > 0.30 && turnover <= 1.0 { // 30-100%
		return 85
	} else if turnover > 1.0 { // >100% (but <$5M liq = risky)
		return 60
	} else if turnover >= 0.02 {
		return 50 + (turnover-0.02)*1666
	} else {
		return turnover * 2500
	}
}

func calculateHolderScore(top10Percentage float64) float64 {
	// Lower concentration = better
	if top10Percentage < 20 {
		return 100
	} else if top10Percentage < 40 {
		return 85
	} else if top10Percentage < 60 {
		return 60
	} else {
		return 30
	}
}

func calculateFragmentationScore(isFragmentationSafe bool) float64 {
	if isFragmentationSafe {
		return 100
	}
	return 40 // Penalty but not elimination
}
