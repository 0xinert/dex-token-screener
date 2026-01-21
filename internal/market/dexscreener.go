// Package market implements a client for fetching liquidity and volume data from DexScreener API.
package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"
)

type DexScreenerClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewDexScreenerClient() *DexScreenerClient {
	return &DexScreenerClient{
		baseURL:    "https://api.dexscreener.com",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPairMetrics fetches liquidity and volume data
func (d *DexScreenerClient) GetPairMetrics(address string) (liquidity, volume float64, isFragmentationSafe bool, largestSingleLiquidityPoolAgeDays float64, err error) {
	url := fmt.Sprintf("%s/token-pairs/v1/bsc/%s", d.baseURL, address)

	resp, err := d.httpClient.Get(url)
	if err != nil {
		return 0, 0, false, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var pairs []models.DexScreenerPair
	if err := json.Unmarshal(body, &pairs); err != nil {
		return 0, 0, false, 0, err
	}

	if len(pairs) == 0 {
		return 0, 0, false, 0, fmt.Errorf("no DEXScreener pairs found for token %s", address)
	}

	// Filter to keep only USDT pairs (case-insensitive address comparison)
	const usdtAddress = "0x55d398326f99059ff775485246999027b3197955"
	usdtPairs := make([]models.DexScreenerPair, 0)
	for _, pair := range pairs {
		if strings.EqualFold(pair.QuoteToken.Address, usdtAddress) {
			usdtPairs = append(usdtPairs, pair)
		}
	}

	if len(usdtPairs) == 0 {
		return 0, 0, false, 0, fmt.Errorf("no USDT pairs found for token %s", address)
	}

	largestSingleLiquidityPool := models.DexScreenerPair{}
	// Aggregate liquidity and volume across all pairs
	for _, pair := range usdtPairs {
		if pair.Liquidity.USD > largestSingleLiquidityPool.Liquidity.USD {
			largestSingleLiquidityPool = pair
		}
		liquidity += pair.Liquidity.USD
		volume += pair.Volume.H24
	}

	if liquidity > 10_000_000 { // If >$10M total, fragmentation OK
		isFragmentationSafe = true
	} else if largestSingleLiquidityPool.Liquidity.USD >= 0.5*liquidity {
		isFragmentationSafe = true
	}

	// fmt.Printf("DEBUG: PairCreatedAt raw: %d\n", largestSingleLiquidityPool.PairCreatedAt)
	// fmt.Printf("DEBUG: Converted to time: %v\n", time.Unix(largestSingleLiquidityPool.PairCreatedAt/1000, 0))
	// Age of largest single liquidity pool in days
	largestSingleLiquidityPoolAgeDays = time.Since(time.Unix(largestSingleLiquidityPool.PairCreatedAt/1000, 0)).Hours() / 24

	return liquidity, volume, isFragmentationSafe, largestSingleLiquidityPoolAgeDays, nil
}
