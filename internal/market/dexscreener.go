// Package market implements a client for fetching liquidity and volume data from DexScreener API.
package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"
)

type DexScreenerClient struct {
	baseURL    string
	httpClient *http.Client
}

type DexScreenerResponse struct {
	Pairs []models.DexScreenerPair `json:"pairs"`
}

func NewDexScreenerClient() *DexScreenerClient {
	return &DexScreenerClient{
		baseURL:    "https://api.dexscreener.com",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPairMetrics fetches liquidity and volume data
func (d *DexScreenerClient) GetPairMetrics(address string) (liquidity, volume float64, fragmentationSafe bool, largetsSingleLiquidityPoolAgeDays float64, err error) {
	url := fmt.Sprintf("%s/token-pairs/v1/bsc/%s", d.baseURL, address)

	resp, err := d.httpClient.Get(url)
	if err != nil {
		return 0, 0, false, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result DexScreenerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, false, 0, err
	}

	if len(result.Pairs) == 0 {
		return 0, 0, false, 0, fmt.Errorf("no DEXScreener pairs found for token %s", address)
	}

	// Filter to keep only USDT pairs
	usdtPairs := make([]models.DexScreenerPair, 0)
	for _, pair := range result.Pairs {
		if pair.QuoteToken.Address == "0x55d398326f99059ff775485246999027b3197955" {
			usdtPairs = append(usdtPairs, pair)
		}
	}
	result.Pairs = usdtPairs

	largetsSingleLiquidityPool := models.DexScreenerPair{}
	// Aggregate liquidity and volume across all pairs
	for _, pair := range result.Pairs {
		if pair.Liquidity.USD > largetsSingleLiquidityPool.Liquidity.USD {
			largetsSingleLiquidityPool = pair
		}
		liquidity += pair.Liquidity.USD
		volume += pair.Volume.H24
	}

	if largetsSingleLiquidityPool.Liquidity.USD >= 0.5*liquidity {
		fragmentationSafe = true
	}
	// Age of largets single liquidity pool in days
	largetsSingleLiquidityPoolAgeDays = time.Since(time.Unix(largetsSingleLiquidityPool.PairCreatedAt, 0)).Hours() / 24

	return liquidity, volume, fragmentationSafe, largetsSingleLiquidityPoolAgeDays, nil
}
