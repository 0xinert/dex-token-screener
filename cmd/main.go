package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/config"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"
	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/scanner"
)

// "github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"

type BasicTokenInfo struct {
	Address  string `json:"contract_address"`
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Decimals int    `json:"decimals"`
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Debug: check if API key is loaded
	if cfg.BscScanAPIKey == "" {
		fmt.Println("WARNING: BSCSCAN_API_KEY environment variable is not set!")
	} else {
		fmt.Printf("API Key loaded (first 8 chars): %s...\n", cfg.BscScanAPIKey[:min(8, len(cfg.BscScanAPIKey))])
	}

	bscScanClient := scanner.NewBscScanClient(cfg.BscScanAPIKey)

	tokenInfos := readTokens("smallTokensList.json")

	// fmt.Printf("Read %v tokens from file.\n", tokenInfos)

	for i, tokenInfo := range tokenInfos {
		// fmt.Printf("[%d/%d] Processing %s (%s)...\n", i+1, len(tokenInfos), tokenInfo.Name, tokenInfo.Address)

		// _ = tokenInfo

		fmt.Printf("[%d/%d] Processing %s...\n", i+1, len(tokenInfos), tokenInfo.Address)
		token := models.Token{
			Address: tokenInfo.Address,
		}
		_ = token

		isVerified, err := bscScanClient.IsContractVerified(tokenInfo.Address)
		if err != nil {
			fmt.Println("Error checking contract verification:", err)
		}

		// Note: GetContractAge requires paid Etherscan API for BSC
		// Skipping age check for now
		// age, err := bscScanClient.GetContractAge(tokenInfo.Address)
		// isContractOldEnough, _ := bscScanClient.IsContractOldEnough(tokenInfo.Address)

		fmt.Printf("Token: %s, Contract verified: %t\n", tokenInfo.Name, isVerified)
		time.Sleep(2 * time.Second)
	}
}

func readTokens(fileName string) []BasicTokenInfo {
	data, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return nil
	}

	var tokens []BasicTokenInfo
	if err := json.Unmarshal(data, &tokens); err != nil {
		fmt.Println("Error parsing JSON:", err)
		return nil
	}
	return tokens
}
