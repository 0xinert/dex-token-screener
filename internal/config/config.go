package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// API Keys
	BscScanAPIKey string
	DatabaseURL   string

	// Thresholds
	MinLiquidityUSD float64
	MinVolume24h    float64
	MaxTop10Holders float64

	// Scoring weights
	LiquidityWeight float64
	VolumeWeight    float64
	HolderWeight    float64

	// Score thresholds
	Threshold float64
}

func Load() *Config {
	// Load .env file if it exists
	_ = godotenv.Load()

	return &Config{
		BscScanAPIKey: os.Getenv("BSCSCAN_API_KEY"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),

		MinLiquidityUSD: getEnvFloat("MIN_LIQUIDITY_USD", 100000),
		MinVolume24h:    getEnvFloat("MIN_VOLUME_24H", 10000),
		MaxTop10Holders: getEnvFloat("MAX_TOP10_HOLDERS", 70),

		LiquidityWeight: 0.40,
		VolumeWeight:    0.35,
		HolderWeight:    0.25,

		Threshold: 70.0,
	}
}

func getEnvFloat(key string, defaultVal float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	f, _ := strconv.ParseFloat(val, 64)
	return f
}
