package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/config"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/contract"
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
}

type Statistics struct {
	TotalTokens       int
	ErrorCount        int
	EvaluatedCount    int
	PassedCount       int
	FailedCount       int
	NoUSDTPairs       int
	NoDexScreenerData int
	NoHoneyPotData    int
	OtherErrors       int
}

func main() {
	cfg := config.Load()

	if cfg.BscScanAPIKey == "" {
		fmt.Println("ERROR: BSCSCAN_API_KEY not set")
		return
	}

	bscScanClient := contract.NewBscScanClient(cfg.BscScanAPIKey)
	dexscreenerClient := market.NewDexScreenerClient()
	honypotClient := market.NewHoneyPotClient()

	tokenInfos := readTokens("tokenData/parsed_10000_BSC_tokens.json")

	// Create output file
	outputFile, err := os.Create(fmt.Sprintf("./results/screening_results_%s.txt", time.Now().Format("2006-01-02_15-04-05")))
	if err != nil {
		fmt.Printf("ERROR: Could not create output file: %v\n", err)
		return
	}
	defer outputFile.Close()

	// Write header
	header := fmt.Sprintf("Token Screening Pipeline - %s\nTotal: %d tokens\n\n", time.Now().Format("2006-01-02 15:04:05"), len(tokenInfos))
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

		// Contract verification
		verified, err := bscScanClient.IsContractVerified(tokenInfo.Address)
		if err != nil {
			errMsg := fmt.Sprintf("  ERROR: %v\n\n", err)
			fmt.Print(errMsg)
			outputFile.WriteString(errMsg)

			result.Status = "ERROR"
			result.ErrorReason = err.Error()
			stats.ErrorCount++
			stats.OtherErrors++
			results = append(results, result)
			continue
		}

		// Liquidity metrics
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

		// If we got here, token was evaluated
		stats.EvaluatedCount++

		// TODO: Implement holder concentration
		holderConc, err := honypotClient.GetTop10HoldersConcentration(tokenInfo.Address)
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
			} else if contains(err.Error(), "no honeyPot data found") {
				stats.NoHoneyPotData++
			} else {
				stats.OtherErrors++
			}

			results = append(results, result)
			continue
		}

		// Calculate score
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

		// Output details
		details := fmt.Sprintf("  Verified: %t | Liq: $%.0f | Vol: $%.0f | Age: %.1fd | Frag: %t | Conc: %.2f%%\n",
			verified, liq, vol, poolAge, fragSafe, holderConc)
		details += fmt.Sprintf("  Score: %.2f (L:%.0f V:%.0f H:%.0f F:%.0f)\n",
			scoreResult.CompositeScore, scoreResult.LiquidityScore, scoreResult.VolumeScore, scoreResult.HolderScore, scoreResult.FragmentationScore)

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

			details += fmt.Sprintf("  Result: SAFE - %s\n\n", status)
		} else {
			stats.FailedCount++
			result.Status = "FAILED"
			details += fmt.Sprintf("  Result: REJECTED - %s\n\n", scoreResult.FailureReasons[0])
		}

		fmt.Print(details)
		outputFile.WriteString(details)
		results = append(results, result)

		time.Sleep(200 * time.Millisecond) // Faster rate limiting
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
	summary := "\n" + string('=') + " SCREENING SUMMARY " + string('=') + "\n\n"
	summary += fmt.Sprintf("Total Tokens Processed: %d\n\n", stats.TotalTokens)

	summary += "Data Availability:\n"
	summary += fmt.Sprintf("  • Tokens with errors: %d (%.1f%%)\n",
		stats.ErrorCount, float64(stats.ErrorCount)/float64(stats.TotalTokens)*100)
	summary += fmt.Sprintf("    - No USDT pairs: %d\n", stats.NoUSDTPairs)
	summary += fmt.Sprintf("    - Not on DexScreener: %d\n", stats.NoDexScreenerData)
	summary += fmt.Sprintf("    - Other errors: %d\n", stats.OtherErrors)
	summary += fmt.Sprintf("  • Tokens evaluated: %d (%.1f%%)\n\n",
		stats.EvaluatedCount, float64(stats.EvaluatedCount)/float64(stats.TotalTokens)*100)

	summary += "Evaluation Results:\n"
	if stats.EvaluatedCount > 0 {
		summary += fmt.Sprintf("  • PASSED: %d (%.1f%% of evaluated)\n",
			stats.PassedCount, float64(stats.PassedCount)/float64(stats.EvaluatedCount)*100)
		summary += fmt.Sprintf("  • FAILED: %d (%.1f%% of evaluated)\n",
			stats.FailedCount, float64(stats.FailedCount)/float64(stats.EvaluatedCount)*100)
	} else {
		summary += "  No tokens were evaluated\n"
	}

	summary += fmt.Sprintf("\nFinal Whitelisted Tokens: %d/%d (%.1f%% of total)\n",
		stats.PassedCount, stats.TotalTokens, float64(stats.PassedCount)/float64(stats.TotalTokens)*100)

	return summary
}

func generateDetailedBreakdown(results []TokenResult, stats Statistics) string {
	breakdown := "\n\n" + string('=') + " DETAILED BREAKDOWN " + string('=') + "\n\n"

	// Passed tokens
	breakdown += fmt.Sprintf("PASSED TOKENS (%d):\n", stats.PassedCount)
	breakdown += "Symbol | Address | Score | Liquidity | Volume\n"
	breakdown += string('-') + "\n"
	for _, r := range results {
		if r.Status == "PASSED" {
			breakdown += fmt.Sprintf("%s | %s | %.2f | $%.0f | $%.0f\n",
				r.Symbol, r.Address, r.Score, r.Liquidity, r.Volume)
		}
	}

	// Failed tokens
	breakdown += fmt.Sprintf("\n\nFAILED TOKENS (%d):\n", stats.FailedCount)
	breakdown += "Symbol | Address | Reason\n"
	breakdown += string('-') + "\n"
	for _, r := range results {
		if r.Status == "FAILED" {
			reason := "Unknown"
			if len(r.FailureReasons) > 0 {
				reason = r.FailureReasons[0]
			}
			breakdown += fmt.Sprintf("%s | %s | %s\n", r.Symbol, r.Address, reason)
		}
	}

	// Error tokens (sample)
	breakdown += fmt.Sprintf("\n\nERROR TOKENS (showing first 50 of %d):\n", stats.ErrorCount)
	breakdown += "Symbol | Address | Error\n"
	breakdown += string('-') + "\n"
	count := 0
	for _, r := range results {
		if r.Status == "ERROR" && count < 50 {
			breakdown += fmt.Sprintf("%s | %s | %s\n", r.Symbol, r.Address, r.ErrorReason)
			count++
		}
	}

	return breakdown
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
