// Package contract implements BscScan API client to fetch and verify token contracts
package contract

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type BscScanClient struct {
	apikey     string
	baseURL    string
	httpClient *http.Client
}

type ContractSourceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []struct {
		SourceCode   string `json:"SourceCode"`
		ABI          string `json:"ABI"`
		ContractName string `json:"ContractName"`
		Proxy        string `json:"Proxy"`
	} `json:"result"`
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

type TokenTotalSupplyResponse struct {
	Status string `json:"status"`
	Result string `json:"result"`
}

func NewBscScanClient(apiKey string) *BscScanClient {
	return &BscScanClient{
		apikey:     apiKey,
		baseURL:    "https://api.etherscan.io/v2/api",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// IsContractVerified checks if the contract has source code and ABI and proxy is not set
func (c *BscScanClient) IsContractVerified(contractAddress string) (bool, error) {
	url := fmt.Sprintf("%s?chainid=56&module=contract&action=getsourcecode&address=%s&apikey=%s",
		c.baseURL, contractAddress, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching contract source:", err)
		return false, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// First check if it's an error response
	var errResp APIErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Status == "0" {
		return false, fmt.Errorf("API error: %s", errResp.Result)
	}

	var result ContractSourceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return false, err
	}

	return result.Status == "1" && len(result.Result) > 0 && result.Result[0].SourceCode != "" && result.Result[0].ABI != "" && result.Result[0].Proxy == "0", nil
}

// IscontractOldEnough checks if the contract is older than 7 days
func (c *BscScanClient) IsContractOldEnough(contractAddress string) (bool, error) {
	deplodAt, err := c.GetContractAge(contractAddress)
	if err != nil {
		fmt.Println("Error calling GetContractAge funtion:", err)
		return false, err
	}

	age := time.Since(deplodAt)

	if age < 7*24*time.Hour {
		return false, nil
	}

	return true, nil
}

func (c *BscScanClient) GetContractAge(contractAddress string) (time.Time, error) {
	url := fmt.Sprintf("%s?chainid=56&module=contract&action=getcontractcreation&contractaddresses=%s&apikey=%s",
		c.baseURL, contractAddress, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching contract creation:", err)
		return time.Time{}, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println("GetContractAge raw response:", string(body))

	// First check if it's an error response
	var errResp APIErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Status == "0" {
		return time.Time{}, fmt.Errorf("API error: %s", errResp.Result)
	}

	var result TokenCreationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling TokenCreationResponse response:", err)
		return time.Time{}, err
	}

	if len(result.Result) == 0 {
		return time.Time{}, nil
	}

	timestamp, err := strconv.ParseInt(result.Result[0].TimeStamp, 10, 64)
	if err != nil {
		fmt.Println("Error parsing timestamp:", err)
		return time.Time{}, err
	}

	deployTime := time.Unix(timestamp, 0)

	return deployTime, nil
}

func (c *BscScanClient) GetTotalSupply(contractAddress string) (float64, error) {
	url := fmt.Sprintf("%s?chainid=56&module=stats&action=tokensupply&contractaddress=%s&apikey=%s",
		c.baseURL, contractAddress, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching token holders:", err)
		return 0, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result TokenTotalSupplyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling TokenTotalSupplyResponse response:", err)
		return 0, err
	}

	if len(result.Result) == 0 {
		return 0, nil
	}

	return strconv.ParseFloat(result.Result, 64)
}

// ways i can easily check the age of a given bsc dex token just by using it's contract address:
//
// 1. If Dexscreener pairCreatedAt exists → use it
// 2. Else earliest Transfer mint tx timestamp
// 3. Else PancakeSwap pair creation block
// 4. Label confidence level
