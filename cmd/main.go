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

func main() {
	cfg := config.Load()

	if cfg.BscScanAPIKey == "" {
		fmt.Println("ERROR: BSCSCAN_API_KEY not set")
		return
	}

	bscScanClient := contract.NewBscScanClient(cfg.BscScanAPIKey)
	dexscreenerClient := market.NewDexScreenerClient()

	// tokenInfos := readTokens("smallGoodTokensList.json")
	tokenInfos := readTokens("output.json")

	fmt.Printf("Token Screening Pipeline\n")
	fmt.Printf("Total: %d tokens\n\n", len(tokenInfos))

	safeCount := 0

	for i, tokenInfo := range tokenInfos {
		fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(tokenInfos), tokenInfo.Symbol, tokenInfo.Address)

		// Contract verification
		verified, err := bscScanClient.IsContractVerified(tokenInfo.Address)
		if err != nil {
			fmt.Printf("  ERROR: %v\n\n", err)
			continue
		}

		// Liquidity metrics
		liq, vol, fragSafe, poolAge, err := dexscreenerClient.GetPairMetrics(tokenInfo.Address)
		if err != nil {
			fmt.Printf("  ERROR: %v\n\n", err)
			continue
		}

		// TODO: Implement holder concentration
		holderConc := 20.0

		// Calculate score
		result, safe := scoring.Scorer(
			verified,
			liq,
			vol,
			holderConc,
			fragSafe,
			poolAge,
		)

		// Output
		fmt.Printf("  Verified: %t | Liq: $%.0f | Vol: $%.0f | Age: %.1fd | Frag: %t\n",
			verified, liq, vol, poolAge, fragSafe)
		fmt.Printf("  Score: %.2f (L:%.0f V:%.0f H:%.0f F:%.0f)\n",
			result.CompositeScore,
			result.LiquidityScore,
			result.VolumeScore,
			result.HolderScore,
			result.FragmentationScore,
		)

		if safe {
			safeCount++
			status := "VISIBLE"
			if result.CompositeScore >= cfg.FeaturedThreshold {
				status = "FEATURED"
			}

			// Add fragmentation warning
			if !fragSafe {
				status += " ⚠️ High slippage risk"
			}

			fmt.Printf("  Result: SAFE - %s\n\n", status)
		} else {
			fmt.Printf("  Result: REJECTED - %s\n\n", result.FailureReasons[0])
		}

		time.Sleep(2 * time.Second)
	}

	fmt.Printf("Summary: %d/%d passed (%.1f%%)\n",
		safeCount, len(tokenInfos), float64(safeCount)/float64(len(tokenInfos))*100)
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
