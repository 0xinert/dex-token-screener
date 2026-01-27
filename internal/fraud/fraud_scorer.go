package fraud

import (
	"fmt"
	"strings"
)

// FraudResult represents the aggregated fraud analysis verdict
type FraudResult struct {
	// Final verdict
	IsSafe          bool
	IsHoneypot      bool
	RejectionReason string

	// Aggregated metrics
	MaxBuyTax      float64
	MaxSellTax     float64
	MaxTransferTax float64
	TotalTax       float64 // Buy + Sell

	// Risk metrics
	HolderFailRate     float64 // From Honeypot.is
	Top10Concentration float64 // From GoPlus
	CreatorPercent     float64 // From GoPlus

	// Risk flags (for logging/penalties)
	RiskFactors []string
	RiskScore   int // 0-100 (0=safe, 100=maximum risk)

	// Contract risks
	IsProxy      bool
	IsOpenSource bool
	HasOwner     bool

	// Raw data (for debugging)
	HoneypotData *HoneypotData
	GoPlusData   *GoPlusData
}

// Thresholds for fraud detection
const (
	MaxAcceptableTax        = 15.0 // 15% total tax (buy + sell)
	MaxHolderFailRate       = 0.10 // 10% of holders failing to sell
	MinHolderSampleSize     = 100  // Minimum holders to trust fail rate
	MaxCreatorPercent       = 0.20 // 20% creator holdings
	HighTaxWarningThreshold = 10.0 // 10% total tax triggers warning
)

// AggregateFraudCheck combines Honeypot.is and GoPlus results into a single verdict
func AggregateFraudCheck(honeypot *HoneypotData, goplus *GoPlusData) *FraudResult {
	result := &FraudResult{
		HoneypotData: honeypot,
		GoPlusData:   goplus,
		IsSafe:       true, // Assume safe until proven otherwise
	}

	// ==== HARD REJECTS (Any of these = instant fail) ====

	// 1. Honeypot.is detected honeypot
	if honeypot.IsHoneypot {
		result.IsHoneypot = true
		result.IsSafe = false
		result.RejectionReason = fmt.Sprintf("Honeypot detected: %s", honeypot.HoneypotReason)
		return result
	}

	// 2. GoPlus detected cannot buy
	if goplus.CannotBuy {
		result.IsHoneypot = true
		result.IsSafe = false
		result.RejectionReason = "Cannot buy token (GoPlus)"
		return result
	}

	// 3. GoPlus detected cannot sell all (partial honeypot)
	if goplus.CannotSellAll {
		result.IsHoneypot = true
		result.IsSafe = false
		result.RejectionReason = "Cannot sell all tokens - partial honeypot (GoPlus)"
		return result
	}

	// 4. Holder fail rate check (CRITICAL - this caught the AVL scam!)
	if honeypot.TotalHolders >= MinHolderSampleSize {
		result.HolderFailRate = honeypot.FailRate

		if honeypot.FailRate > MaxHolderFailRate {
			result.IsHoneypot = true
			result.IsSafe = false
			result.RejectionReason = fmt.Sprintf(
				"High holder fail rate: %.1f%% (%d/%d holders cannot sell)",
				honeypot.FailRate*100,
				honeypot.FailedSells,
				honeypot.TotalHolders,
			)
			return result
		}
	}

	// 5. Tax aggregation (take MAX from both APIs for safety)
	result.MaxBuyTax = max(honeypot.BuyTax, goplus.BuyTax)
	result.MaxSellTax = max(honeypot.SellTax, goplus.SellTax)
	result.MaxTransferTax = max(honeypot.TransferTax, goplus.TransferTax)
	result.TotalTax = result.MaxBuyTax + result.MaxSellTax

	if result.TotalTax > MaxAcceptableTax {
		result.IsSafe = false
		result.RejectionReason = fmt.Sprintf(
			"Excessive tax: %.1f%% (buy: %.1f%%, sell: %.1f%%)",
			result.TotalTax,
			result.MaxBuyTax,
			result.MaxSellTax,
		)
		return result
	}

	// 6. Creator has other honeypot tokens
	if goplus.HoneypotWithCreator {
		result.IsHoneypot = true
		result.IsSafe = false
		result.RejectionReason = "Creator has deployed other honeypot tokens (GoPlus)"
		return result
	}

	// ==== RISK FACTORS (Warnings, not rejections) ====
	riskFactors := []string{}

	// Proxy contract (upgradeable = owner can change code)
	if honeypot.IsProxy || goplus.IsProxy {
		result.IsProxy = true
		riskFactors = append(riskFactors, "proxy_contract")
	}

	// Not open source (can't verify code)
	if !honeypot.IsOpenSource || !goplus.IsOpenSource {
		result.IsOpenSource = false
		riskFactors = append(riskFactors, "not_open_source")
	}

	// Owner not renounced
	if goplus.HasOwner {
		result.HasOwner = true
		riskFactors = append(riskFactors, "owner_not_renounced")
	}

	// High creator holdings
	result.CreatorPercent = goplus.CreatorPercent
	if goplus.CreatorPercent > MaxCreatorPercent {
		riskFactors = append(riskFactors, fmt.Sprintf(
			"high_creator_holdings_%.1f%%",
			goplus.CreatorPercent*100,
		))
	}

	// Centralized liquidity (single LP holder)
	if goplus.LPHolderCount == 1 {
		riskFactors = append(riskFactors, "centralized_liquidity")
	}

	// High tax (not rejected, but noteworthy)
	if result.TotalTax > HighTaxWarningThreshold {
		riskFactors = append(riskFactors, fmt.Sprintf(
			"high_tax_%.1f%%",
			result.TotalTax,
		))
	}

	// Moderate holder fail rate (5-10%)
	if honeypot.TotalHolders >= MinHolderSampleSize && honeypot.FailRate > 0.05 {
		riskFactors = append(riskFactors, fmt.Sprintf(
			"moderate_fail_rate_%.1f%%",
			honeypot.FailRate*100,
		))
	}

	// Store top 10 concentration from GoPlus
	result.Top10Concentration = goplus.Top10Concentration

	result.RiskFactors = riskFactors
	result.RiskScore = calculateRiskScore(result)

	// Token passed all hard checks
	result.IsSafe = true
	return result
}

// calculateRiskScore assigns a 0-100 risk score based on risk factors
// 0 = safest, 100 = highest risk (but still passed hard checks)
func calculateRiskScore(result *FraudResult) int {
	score := 0

	// Each risk factor adds points
	if result.IsProxy {
		score += 15
	}
	if !result.IsOpenSource {
		score += 10
	}
	if result.HasOwner {
		score += 10
	}
	if result.CreatorPercent > MaxCreatorPercent {
		score += 20
	}
	if result.TotalTax > HighTaxWarningThreshold {
		score += 15
	}
	if result.HolderFailRate > 0.05 { // 5-10% fail rate
		score += 20
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return score
}

// GetRiskSummary returns a human-readable summary of risk factors
func (f *FraudResult) GetRiskSummary() string {
	if len(f.RiskFactors) == 0 {
		return "No risk factors detected"
	}
	return strings.Join(f.RiskFactors, ", ")
}

// Helper function
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
