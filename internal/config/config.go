// Package config provides configuration curerntly for bscscan and dexscreener
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
	MinLiquidityUSD             float64
	MinVolume24h                float64
	MaxTop10HolderConcentration float64

	// Scoring weights
	LiquidityWeight     float64
	VolumeWeight        float64
	HolderWeight        float64
	FragmentationWeight float64

	// Score thresholds
	FeaturedThreshold float64 // ADD THIS
	VisibleThreshold  float64 // ADD THIS
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		BscScanAPIKey: os.Getenv("BSCSCAN_API_KEY"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),

		MinLiquidityUSD:             getEnvFloat("MIN_LIQUIDITY_USD", 100000),
		MinVolume24h:                getEnvFloat("MIN_VOLUME_24H", 10000),
		MaxTop10HolderConcentration: getEnvFloat("MAX_TOP10_HOLDERS", 70),

		LiquidityWeight:     0.35,
		VolumeWeight:        0.30,
		HolderWeight:        0.25,
		FragmentationWeight: 0.10,

		FeaturedThreshold: 70.0, // ADD THIS
		VisibleThreshold:  50.0, // ADD THIS
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
