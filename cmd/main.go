package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/config"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/contract"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/fraud"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/market"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/scoring"
)

type BasicTokenInfo struct {
	Address  string `json:"contract_address"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

type TokenResult struct {
	Symbol         string
	Address        string
	Status         string // "PASSED", "FAILED", "ERROR"
	ErrorReason    string
	Score          float64
	Liquidity      float64
	Volume         float64
	Age            float64
	Fragmented     bool
	Concentration  float64
	FailureReasons []string
	RiskFactors    []string // NEW: Fraud risk factors
}

type Statistics struct {
	TotalTokens       int
	ErrorCount        int
	EvaluatedCount    int
	PassedCount       int
	FailedCount       int
	NoUSDTPairs       int
	NoDexScreenerData int
	FraudAPIErrors    int // NEW: Track fraud API failures
	HoneypotRejected  int // NEW: Track honeypot rejections
	OtherErrors       int
}

func main() {
	cfg := config.Load()

	if cfg.BscScanAPIKey == "" {
		fmt.Println("ERROR: BSCSCAN_API_KEY not set")
		return
	}

	// Initialize clients
	bscScanClient := contract.NewBscScanClient(cfg.BscScanAPIKey)
	dexscreenerClient := market.NewDexScreenerClient()
	honeypotConcentrationClient := market.NewHoneyPotClient()
	honeypotFraudClient := fraud.NewHoneypotClient()
	goplusFraudClient := fraud.NewGoPlusClient()

	tokenInfos := readTokens("tokenData/parsed_10000_BSC_tokens.json")

	// Create output file
	outputFile, err := os.Create(fmt.Sprintf("./results/screening_results_%s.txt", time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		fmt.Printf("ERROR: Could not create output file: %v\n", err)
		return
	}
	defer outputFile.Close()

	// Write header
	header := fmt.Sprintf("BSC Token Screening Pipeline - %s\nTotal: %d tokens\n\n", time.Now().Format("2006-01-02 15:04:05"), len(tokenInfos))
	fmt.Print(header)
	outputFile.WriteString(header)

	stats := Statistics{
		TotalTokens: len(tokenInfos),
	}
	var results []TokenResult

	for i, tokenInfo := range tokenInfos {
		consoleOutput := fmt.Sprintf("[%d/%d] %s (%s)\n", i+1, len(tokenInfos), tokenInfo.Symbol, tokenInfo.Address)
		fmt.Print(consoleOutput)
		outputFile.WriteString(consoleOutput)

		result := TokenResult{
			Symbol:  tokenInfo.Symbol,
			Address: tokenInfo.Address,
		}

		// ===== STEP 1: DEXSCREENER - CHECK USDT PAIRS + LIQ/VOL =====
		liq, vol, fragSafe, poolAge, err := dexscreenerClient.GetPairMetrics(tokenInfo.Address)
		if err != nil {
			errMsg := fmt.Sprintf("  ERROR: %v\n\n", err)
			fmt.Print(errMsg)
			outputFile.WriteString(errMsg)

			result.Status = "ERROR"
			result.ErrorReason = err.Error()
			stats.ErrorCount++

			// Categorize error type
			if contains(err.Error(), "no USDT pairs") {
				stats.NoUSDTPairs++
			} else if contains(err.Error(), "no DEXScreener pairs") {
				stats.NoDexScreenerData++
			} else {
				stats.OtherErrors++
			}

			results = append(results, result)
			continue
		}

		// Token has USDT pairs and is on DexScreener - can be evaluated
		stats.EvaluatedCount++

		// ===== STEP 2: CHECK LIQ/VOL THRESHOLDS BEFORE FURTHER API CALLS =====
		if liq < cfg.MinLiquidityUSD || vol < cfg.MinVolume24h {
			failMsg := fmt.Sprintf("  REJECTED: Below thresholds (Liq: $%.0f, Vol: $%.0f)\n\n", liq, vol)
			fmt.Print(failMsg)
			outputFile.WriteString(failMsg)

			result.Status = "FAILED"
			result.Liquidity = liq
			result.Volume = vol
			result.FailureReasons = []string{
				fmt.Sprintf("Below minimum thresholds (Liq: $%.0f < $%.0f, Vol: $%.0f < $%.0f)",
					liq, cfg.MinLiquidityUSD, vol, cfg.MinVolume24h),
			}
			stats.FailedCount++
			results = append(results, result)
			continue
		}

		// ===== STEP 3: HOLDER CONCENTRATION =====
		holderConc, err := honeypotConcentrationClient.GetTop10HoldersConcentration(tokenInfo.Address)
		if err != nil {
			errMsg := fmt.Sprintf("  ERROR: Holder concentration check failed: %v\n\n", err)
			fmt.Print(errMsg)
			outputFile.WriteString(errMsg)

			result.Status = "ERROR"
			result.ErrorReason = err.Error()
			stats.ErrorCount++
			stats.OtherErrors++
			results = append(results, result)
			continue
		}

		// ===== STEP 4: CONTRACT VERIFICATION =====
		verified, err := bscScanClient.IsContractVerified(tokenInfo.Address)
		if err != nil {
			errMsg := fmt.Sprintf("  ERROR: BscScan verification check failed: %v\n\n", err)
			fmt.Print(errMsg)
			outputFile.WriteString(errMsg)

			result.Status = "ERROR"
			result.ErrorReason = err.Error()
			stats.ErrorCount++
			stats.OtherErrors++
			results = append(results, result)
			continue
		}

		if !verified {
			failMsg := fmt.Sprintf("  REJECTED: Contract not verified\n\n")
			fmt.Print(failMsg)
			outputFile.WriteString(failMsg)

			result.Status = "FAILED"
			result.Liquidity = liq
			result.Volume = vol
			result.FailureReasons = []string{"Contract not verified"}
			stats.FailedCount++
			results = append(results, result)
			continue
		}

		// ===== STEP 5: FRAUD DETECTION (HONEYPOT + GOPLUS) =====
		honeypotData, err := honeypotFraudClient.CheckToken(tokenInfo.Address)
		if err != nil {
			errMsg := fmt.Sprintf("  ERROR: Honeypot API failed: %v\n\n", err)
			fmt.Print(errMsg)
			outputFile.WriteString(errMsg)

			result.Status = "ERROR"
			result.ErrorReason = fmt.Sprintf("Honeypot API: %v", err)
			stats.ErrorCount++
			stats.FraudAPIErrors++
			results = append(results, result)
			continue
		}

		goplusData, err := goplusFraudClient.CheckToken(tokenInfo.Address)
		if err != nil {
			warnMsg := fmt.Sprintf("  WARNING: GoPlus unavailable: %v\n", err)
			fmt.Print(warnMsg)
			outputFile.WriteString(warnMsg)

			// Create safe default (assume best case)
			goplusData = &fraud.GoPlusData{
				IsOpenSource:       true,       // Already verified on BscScan
				Top10Concentration: holderConc, // Will get from Honeypot API
			}
		}

		// Aggregate fraud results
		fraudResult := fraud.AggregateFraudCheck(honeypotData, goplusData)

		// If fraud detected, REJECT immediately (don't even score)
		if !fraudResult.IsSafe {
			fraudMsg := fmt.Sprintf("  REJECTED: %s\n", fraudResult.RejectionReason)
			if len(fraudResult.RiskFactors) > 0 {
				fraudMsg += fmt.Sprintf("  Risk Factors: %v\n", fraudResult.RiskFactors)
			}
			fraudMsg += "\n"
			fmt.Print(fraudMsg)
			outputFile.WriteString(fraudMsg)

			result.Status = "FAILED"
			result.Liquidity = liq
			result.Volume = vol
			result.FailureReasons = []string{fraudResult.RejectionReason}
			result.RiskFactors = fraudResult.RiskFactors
			stats.FailedCount++
			if fraudResult.IsHoneypot {
				stats.HoneypotRejected++
			}
			results = append(results, result)
			continue
		}

		// ===== STEP 6: CALCULATE SCORE =====
		scoreResult, safe := scoring.Scorer(
			verified,
			liq,
			vol,
			holderConc,
			fragSafe,
			poolAge,
		)

		// Populate result
		result.Liquidity = liq
		result.Volume = vol
		result.Age = poolAge
		result.Fragmented = !fragSafe
		result.Concentration = holderConc
		result.Score = scoreResult.CompositeScore
		result.FailureReasons = scoreResult.FailureReasons
		result.RiskFactors = fraudResult.RiskFactors // Include fraud risk factors

		// Output details
		details := fmt.Sprintf("  Verified: %t | Liq: $%.0f | Vol: $%.0f | Age: %.1fd | Frag: %t | Conc: %.2f%%\n",
			verified, liq, vol, poolAge, fragSafe, holderConc)
		details += fmt.Sprintf("  Score: %.2f (L:%.0f V:%.0f H:%.0f F:%.0f)\n",
			scoreResult.CompositeScore, scoreResult.LiquidityScore, scoreResult.VolumeScore, scoreResult.HolderScore, scoreResult.FragmentationScore)

		// Add fraud risk factors if any
		if len(fraudResult.RiskFactors) > 0 {
			details += fmt.Sprintf("  Fraud Risk: %v (Score: %d/100)\n", fraudResult.RiskFactors, fraudResult.RiskScore)
		}

		if safe {
			stats.PassedCount++
			result.Status = "PASSED"

			status := "VISIBLE"
			if scoreResult.CompositeScore >= cfg.FeaturedThreshold {
				status = "FEATURED"
			}

			if !fragSafe {
				status += " ⚠️ High slippage risk"
			}

			details += fmt.Sprintf("  Result: PASSED - %s\n\n", status)
		} else {
			stats.FailedCount++
			result.Status = "FAILED"
			details += fmt.Sprintf("  Result: REJECTED - %s\n\n", scoreResult.FailureReasons[0])
		}

		fmt.Print(details)
		outputFile.WriteString(details)
		results = append(results, result)

		// Rate limiting (adjust based on API limits)
		time.Sleep(2 * time.Second)
	}

	// Generate summary
	summary := generateSummary(stats)
	fmt.Print(summary)
	outputFile.WriteString(summary)

	// Write detailed breakdown
	breakdown := generateDetailedBreakdown(results, stats)
	outputFile.WriteString(breakdown)

	fmt.Printf("\nResults saved to: %s\n", outputFile.Name())
}

func generateSummary(stats Statistics) string {
	summary := "\n" + repeatChar('=', 60) + "\n"
	summary += "                  SCREENING SUMMARY\n"
	summary += repeatChar('=', 60) + "\n\n"
	summary += fmt.Sprintf("Total Tokens Processed: %d\n\n", stats.TotalTokens)

	summary += "Data Availability:\n"
	summary += fmt.Sprintf("  • Tokens with errors: %d (%.1f%%)\n",
		stats.ErrorCount, float64(stats.ErrorCount)/float64(stats.TotalTokens)*100)
	summary += fmt.Sprintf("    - No USDT pairs: %d\n", stats.NoUSDTPairs)
	summary += fmt.Sprintf("    - Not on DexScreener: %d\n", stats.NoDexScreenerData)
	summary += fmt.Sprintf("    - Fraud API errors: %d\n", stats.FraudAPIErrors)
	summary += fmt.Sprintf("    - Other errors: %d\n", stats.OtherErrors)
	summary += fmt.Sprintf("  • Tokens evaluated: %d (%.1f%%)\n\n",
		stats.EvaluatedCount, float64(stats.EvaluatedCount)/float64(stats.TotalTokens)*100)

	summary += "Evaluation Results:\n"
	if stats.EvaluatedCount > 0 {
		summary += fmt.Sprintf("  • PASSED: %d (%.1f%% of evaluated)\n",
			stats.PassedCount, float64(stats.PassedCount)/float64(stats.EvaluatedCount)*100)
		summary += fmt.Sprintf("  • FAILED: %d (%.1f%% of evaluated)\n",
			stats.FailedCount, float64(stats.FailedCount)/float64(stats.EvaluatedCount)*100)
		if stats.HoneypotRejected > 0 {
			summary += fmt.Sprintf("    - Honeypot/Fraud: %d\n", stats.HoneypotRejected)
		}
	} else {
		summary += "  No tokens were evaluated\n"
	}

	summary += fmt.Sprintf("\nFinal Whitelisted Tokens: %d/%d (%.1f%% of total)\n",
		stats.PassedCount, stats.TotalTokens, float64(stats.PassedCount)/float64(stats.TotalTokens)*100)
	summary += repeatChar('=', 60) + "\n"

	return summary
}

func generateDetailedBreakdown(results []TokenResult, stats Statistics) string {
	var breakdown strings.Builder
	breakdown.WriteString("\n\n" + repeatChar('=', 60) + "\n")
	breakdown.WriteString("                 DETAILED BREAKDOWN\n")
	breakdown.WriteString(repeatChar('=', 60) + "\n\n")

	// Passed tokens
	breakdown.WriteString(fmt.Sprintf("PASSED TOKENS (%d):\n", stats.PassedCount))
	breakdown.WriteString(repeatChar('-', 60) + "\n")
	breakdown.WriteString(fmt.Sprintf("%-10s | %-42s | Score | Liquidity\n", "Symbol", "Address"))
	breakdown.WriteString(repeatChar('-', 60) + "\n")
	for _, r := range results {
		if r.Status == "PASSED" {
			breakdown.WriteString(fmt.Sprintf("%-10s | %s | %.2f | $%.0f\n",
				truncate(r.Symbol, 10), r.Address, r.Score, r.Liquidity))
		}
	}

	// Failed tokens
	breakdown.WriteString(fmt.Sprintf("\n\nFAILED TOKENS (%d):\n", stats.FailedCount))
	breakdown.WriteString(repeatChar('-', 60) + "\n")
	breakdown.WriteString(fmt.Sprintf("%-10s | %-42s | Reason\n", "Symbol", "Address"))
	breakdown.WriteString(repeatChar('-', 60) + "\n")
	for _, r := range results {
		if r.Status == "FAILED" {
			reason := "Unknown"
			if len(r.FailureReasons) > 0 {
				reason = truncate(r.FailureReasons[0], 40)
			}
			breakdown.WriteString(fmt.Sprintf("%-10s | %s | %s\n", truncate(r.Symbol, 10), r.Address, reason))
		}
	}

	// Error tokens (sample)
	breakdown.WriteString(fmt.Sprintf("\n\nERROR TOKENS (showing first 50 of %d):\n", stats.ErrorCount))
	breakdown.WriteString(repeatChar('-', 60) + "\n")
	breakdown.WriteString(fmt.Sprintf("%-10s | %-42s | Error\n", "Symbol", "Address"))
	breakdown.WriteString(repeatChar('-', 60) + "\n")
	count := 0
	for _, r := range results {
		if r.Status == "ERROR" && count < 50 {
			breakdown.WriteString(fmt.Sprintf("%-10s | %s | %s\n",
				truncate(r.Symbol, 10), r.Address, truncate(r.ErrorReason, 40)))
			count++
		}
	}

	return breakdown.String()
}

func readTokens(fileName string) []BasicTokenInfo {
	data, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}

	var tokens []BasicTokenInfo
	json.Unmarshal(data, &tokens)
	return tokens
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func repeatChar(char rune, count int) string {
	var result strings.Builder
	for range count {
		result.WriteString(string(char))
	}
	return result.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Key Improvement : Optimized Pipeline Order
// 1. DexScreener → Check USDT pairs exist
// 2. Check liq/vol thresholds → Skip API calls if below
// 3. Holder concentrationpass
// 4. BscScan → Contract verified
// 5. Fraud APIs → Honeypot + GoPlus → If all pass
// 6. Scoring → Final score
