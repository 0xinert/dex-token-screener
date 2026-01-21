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
		baseURL:    "https://api.dexscreener.com/latest/dex/tokens",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetLiquidityMetrics fetches liquidity and volume data
func (d *DexScreenerClient) GetLiquidityMetrics(address string) (liquidity, volume float64, err error) {
	url := fmt.Sprintf("%s/%s", d.baseURL, address)

	resp, err := d.httpClient.Get(url)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result DexScreenerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, err
	}

	if len(result.Pairs) == 0 {
		return 0, 0, fmt.Errorf("no pairs found")
	}

	// Aggregate liquidity and volume across all pairs
	for _, pair := range result.Pairs {
		liquidity += pair.Liquidity.USD
		volume += pair.Volume.H24
	}

	return liquidity, volume, nil
}
