// Package fraud implements fraud detection API clients for token screening
package fraud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type HoneypotClient struct {
	baseURL    string
	httpClient *http.Client
}

// HoneypotAPIResponse represents the full API response structure
type HoneypotAPIResponse struct {
	Token struct {
		Name         string `json:"name"`
		Symbol       string `json:"symbol"`
		Address      string `json:"address"`
		TotalHolders int    `json:"totalHolders"`
	} `json:"token"`
	Summary struct {
		Risk      string `json:"risk"`
		RiskLevel int    `json:"riskLevel"`
		Flags     []struct {
			Flag        string `json:"flag"`
			Description string `json:"description"`
			Severity    string `json:"severity"`
		} `json:"flags"`
	} `json:"summary"`
	SimulationSuccess bool `json:"simulationSuccess"`
	HoneypotResult    struct {
		IsHoneypot     bool   `json:"isHoneypot"`
		HoneypotReason string `json:"honeypotReason"`
	} `json:"honeypotResult"`
	SimulationResult struct {
		BuyTax      float64 `json:"buyTax"`
		SellTax     float64 `json:"sellTax"`
		TransferTax float64 `json:"transferTax"`
		BuyGas      string  `json:"buyGas"`
		SellGas     string  `json:"sellGas"`
	} `json:"simulationResult"`
	HolderAnalysis struct {
		Holders    string  `json:"holders"`
		Successful string  `json:"successful"`
		Failed     string  `json:"failed"`
		Siphoned   string  `json:"siphoned"`
		AverageTax float64 `json:"averageTax"`
	} `json:"holderAnalysis"`
	Flags        []string `json:"flags"`
	ContractCode struct {
		OpenSource     bool `json:"openSource"`
		RootOpenSource bool `json:"rootOpenSource"`
		IsProxy        bool `json:"isProxy"`
		HasProxyCalls  bool `json:"hasProxyCalls"`
	} `json:"contractCode"`
}

// HoneypotData represents the extracted fraud-relevant data
type HoneypotData struct {
	// Core detection
	IsHoneypot     bool
	HoneypotReason string
	RiskLevel      int // 0-100

	// Taxes
	BuyTax      float64
	SellTax     float64
	TransferTax float64

	// Holder analysis (CRITICAL - catches sophisticated honeypots)
	TotalHolders    int
	SuccessfulSells int
	FailedSells     int
	FailRate        float64 // Calculated: failed/total

	// Contract info
	IsOpenSource  bool
	IsProxy       bool
	HasProxyCalls bool

	// Flags
	Flags []string
}

func NewHoneypotClient() *HoneypotClient {
	return &HoneypotClient{
		baseURL:    "https://api.honeypot.is",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckToken performs honeypot analysis on a token address
func (h *HoneypotClient) CheckToken(address string) (*HoneypotData, error) {
	url := fmt.Sprintf("%s/v2/IsHoneypot?address=%s&chainID=56", h.baseURL, address)

	resp, err := h.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching honeypot data:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var apiResp HoneypotAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Println("Error unmarshaling honeypot response:", err)
		return nil, err
	}

	// Parse holder analysis strings to ints
	holders, _ := strconv.Atoi(apiResp.HolderAnalysis.Holders)
	successful, _ := strconv.Atoi(apiResp.HolderAnalysis.Successful)
	failed, _ := strconv.Atoi(apiResp.HolderAnalysis.Failed)

	// Calculate fail rate
	var failRate float64
	if holders > 0 {
		failRate = float64(failed) / float64(holders)
	}

	// Extract only the fields we need
	data := &HoneypotData{
		IsHoneypot:      apiResp.HoneypotResult.IsHoneypot,
		HoneypotReason:  apiResp.HoneypotResult.HoneypotReason,
		RiskLevel:       apiResp.Summary.RiskLevel,
		BuyTax:          apiResp.SimulationResult.BuyTax,
		SellTax:         apiResp.SimulationResult.SellTax,
		TransferTax:     apiResp.SimulationResult.TransferTax,
		TotalHolders:    holders,
		SuccessfulSells: successful,
		FailedSells:     failed,
		FailRate:        failRate,
		IsOpenSource:    apiResp.ContractCode.OpenSource,
		IsProxy:         apiResp.ContractCode.IsProxy,
		HasProxyCalls:   apiResp.ContractCode.HasProxyCalls,
		Flags:           apiResp.Flags,
	}

	return data, nil
}
