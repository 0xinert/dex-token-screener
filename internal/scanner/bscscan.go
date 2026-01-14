package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"
)

type BscScanClient struct {
	apikey     string
	baseURL    string
	httpClient *http.Client
}

type ContracttSouurceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []struct {
		SourceCode   string `json:"SourceCode"`
		ABI          string `json:"ABI"`
		ContractName string `json:"ContractName"`
		Proxy        string `json:"Proxy"`
	} `json:"result"`
}

type TokenHoldersResponse struct {
	Status string                 `json:"status"`
	Result []models.BscScanHolder `json:"result"`
}

func NewBscScanClient(apiKey string) *BscScanClient {
	return &BscScanClient{
		apikey:     apiKey,
		baseURL:    "https://api.etherscan.io/v2/apki",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *BscScanClient) IsContractVerified(address string) (bool, error) {
	url := fmt.Sprintf("%s?address=%s&chainId=56&module=contract&apikey=%s&action=getsourcecode",
		c.baseURL, address, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching contract source:", err)
		return false, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ContracttSouurceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return false, err
	}

	return result.Status == "1" && len(result.Result) > 0 && result.Result[0].SourceCode != "", nil
}
