package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/notlelouch/go-interview-practice/DEX-Token-Screener/internal/models"
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

type TokenHoldersResponse struct {
	Status string                 `json:"status"`
	Result []models.BscScanHolder `json:"result"`
}

type TokenTotalSupplyResponse struct {
	Status string `json:"status"`
	Result string `json:"result"`
}

func NewBscScanClient(apiKey string) *BscScanClient {
	return &BscScanClient{
		apikey:     apiKey,
		baseURL:    "https://api.etherscan.io/v2/apki",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *BscScanClient) IsContractVerified(contractAddress string) (bool, error) {
	url := fmt.Sprintf("%s?address=%s&chainId=56&module=contract&apikey=%s&action=getsourcecode",
		c.baseURL, contractAddress, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching contract source:", err)
		return false, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ContractSourceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return false, err
	}

	return result.Status == "1" && len(result.Result) > 0 && result.Result[0].SourceCode != "" && result.Result[0].ABI != "" && result.Result[0].Proxy == "0", nil
}

func (c *BscScanClient) GetTop10HoldersConcentration(contractAddress string) (float64, error) {
	url := fmt.Sprintf("%s?&chainId=56&module=token&action=topholders&contractaddress=%s&offet=10&apikey=%s",
		c.baseURL, contractAddress, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching token holders:", err)
		return 0, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result TokenHoldersResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return 0, err
	}

	if len(result.Result) == 0 {
		return 0, nil
	}

	var totalTop10Balance float64
	for _, holder := range result.Result {
		balance, err := strconv.ParseFloat(holder.Balance, 64)
		if err != nil {
			fmt.Println("Error parsing top 10 holder balance:", err)
		}
		totalTop10Balance += balance
	}

	totalsupply, err := c.GetTotalSupply(contractAddress)
	if err != nil {
		fmt.Println("Error fetching total supply:", err)
		return 0, err
	}

	if totalsupply == 0 {
		return 0, nil
	}

	return (totalTop10Balance / totalsupply) * 100, nil
}

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
	url := fmt.Sprintf("%s?&chainId=56&module=contract&action=getcontractcreation&contractaddress=%s&apikey=%s",
		c.baseURL, contractAddress, c.apikey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching token holders:", err)
		return time.Time{}, err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

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
	url := fmt.Sprintf("%s?&chainId=56&module=state&action=tokensupply&contractaddress=%s&apikey=%s",
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
