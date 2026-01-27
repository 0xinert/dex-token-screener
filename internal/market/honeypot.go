// Package market implements a HoneyPot API client to fetch token concentration data from HoneyPot API.
package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"
)

type HoneyPotClient struct {
	baseURL    string
	httpClient *http.Client
}

type TopTokenHoldersResponse struct {
	TotalSupply string                  `json:"totalSupply"`
	Holders     []models.HoneyPotHolder `json:"holders"`
}

type TokenCreationResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []struct {
		BlockNumber     string `json:"blockNumber"`
		TimeStamp       string `json:"timestamp"`
		ContractCreator string `json:"contractCreator"`
	} `json:"result"`
}

// APIErrorResponse is used when the API returns an error (result is a string)
type APIErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

func NewHoneyPotClient() *HoneyPotClient {
	return &HoneyPotClient{
		baseURL:    "https://api.honeypot.is",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *HoneyPotClient) GetTop10HoldersConcentration(contractAddress string) (float64, error) {
	url := fmt.Sprintf("%s/v1/TopHolders?address=%s&chainID=56", c.baseURL, contractAddress)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching token holders:", err)
		return 0, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result TopTokenHoldersResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return 0, err
	}

	if len(result.Holders) == 0 {
		return 0, nil
	}

	var totalTop10Balance float64
	for _, holder := range result.Holders {
		balance, err := strconv.ParseFloat(holder.Balance, 64)
		if err != nil {
			fmt.Println("Error parsing top 10 holder balance:", err)
		}
		totalTop10Balance += balance
	}

	totalsupply, err := strconv.ParseFloat(result.TotalSupply, 64)
	if err != nil {
		fmt.Println("Error parsing total supply:", err)
		return 0, err
	}

	if totalsupply == 0 {
		return 0, nil
	}

	return (totalTop10Balance / totalsupply) * 100, nil
}
