package models

import "time"

// Token represents a BSC token with all analysis data
type Token struct {
	Address  string `db:"address" json:"address"`
	Name     string `db:"name" json:"name"`
	Symbol   string `db:"symbol" json:"symbol"`
	Decimals int    `db:"decimals" json:"decimals"`

	// Contract verification
	Verified bool `db:"verified" json:"verified"`
	AgeDays  int  `db:"age_days" json:"age_days"`

	// Liquidity metrics
	LiquidityUSD float64 `db:"liquidity_usd" json:"liquidity_usd"`
	Volume24h    float64 `db:"volume_24h" json:"volume_24h"`

	// Holder distribution
	Top10Concentration float64 `db:"top10_holders" json:"top10_holders"`

	// Risk scoring
	CompositeScore float64 `db:"composite_score" json:"composite_score"`
	Status         string  `db:"status" json:"status"` // featured/visible/hidden
	// LiquidityScore float64 `db:"liquidity_score" json:"liquidity_score"`
	// VolumeScore    float64 `db:"volume_score" json:"volume_score"`
	// HolderScore    float64 `db:"holder_score" json:"holder_score"`

	CheckedAt time.Time `db:"checked_at" json:"checked_at"`
}

// DexScreenerPair represents a trading pair from DexScreener
type DexScreenerPair struct {
	Liquidity struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
	Volume struct {
		H24 float64 `json:"h24"`
	} `json:"volume"`
	// ChainID     string `json:"chainId"`
	// PairAddress string `json:"pairAddress"`
	// BaseToken   struct {
	// 	Address string `json:"address"`
	// 	Name    string `json:"name"`
	// 	Symbol  string `json:"symbol"`
	// } `json:"baseToken"`
}

// BscScanHolder represents token holder data
type BscScanHolder struct {
	Address string `json:"TokenHolderAddress"`
	Balance string `json:"TokenHolderQuantity"`
}
