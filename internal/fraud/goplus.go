package fraud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type GoPlusClient struct {
	baseURL    string
	httpClient *http.Client
}

// GoPlusAPIResponse represents the full API response structure
type GoPlusAPIResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  map[string]struct {
		BuyTax         string `json:"buy_tax"`
		SellTax        string `json:"sell_tax"`
		TransferTax    string `json:"transfer_tax"`
		CannotBuy      string `json:"cannot_buy"`
		CannotSellAll  string `json:"cannot_sell_all"`
		CreatorAddress string `json:"creator_address"`
		CreatorBalance string `json:"creator_balance"`
		CreatorPercent string `json:"creator_percent"`
		HolderCount    string `json:"holder_count"`
		Holders        []struct {
			Address  string `json:"address"`
			Balance  string `json:"balance"`
			Percent  string `json:"percent"`
			IsLocked int    `json:"is_locked"`
		} `json:"holders"`
		HoneypotWithCreator string `json:"honeypot_with_same_creator"`
		IsInDex             string `json:"is_in_dex"`
		IsOpenSource        string `json:"is_open_source"`
		IsProxy             string `json:"is_proxy"`
		LPHolderCount       string `json:"lp_holder_count"`
		OwnerAddress        string `json:"owner_address"`
		TokenName           string `json:"token_name"`
		TokenSymbol         string `json:"token_symbol"`
		TotalSupply         string `json:"total_supply"`
	} `json:"result"`
}

// GoPlusData represents the extracted fraud-relevant data
type GoPlusData struct {
	// Core detection
	BuyTax        float64
	SellTax       float64
	TransferTax   float64
	CannotBuy     bool
	CannotSellAll bool

	// Creator risk
	CreatorPercent      float64
	HoneypotWithCreator bool

	// Holder data
	HolderCount        int
	Top10Concentration float64 // Calculated from holders array

	// Contract info
	IsProxy      bool
	IsOpenSource bool
	HasOwner     bool // True if owner_address exists and not null

	// LP info
	LPHolderCount int
}

func NewGoPlusClient() *GoPlusClient {
	return &GoPlusClient{
		baseURL:    "https://api.gopluslabs.io",
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// CheckToken performs security analysis on a token address
func (g *GoPlusClient) CheckToken(address string) (*GoPlusData, error) {
	url := fmt.Sprintf("%s/api/v1/token_security/56?contract_addresses=%s", g.baseURL, address)

	resp, err := g.httpClient.Get(url)
	if err != nil {
		fmt.Println("Error fetching GoPlus data:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var apiResp GoPlusAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		fmt.Println("Error unmarshaling GoPlus response:", err)
		return nil, err
	}

	// Check if we got a valid response
	if apiResp.Code != 1 {
		return nil, fmt.Errorf("GoPlus API error: %s", apiResp.Message)
	}

	// Get the token data (result is a map with token address as key)
	tokenData, exists := apiResp.Result[address]
	if !exists {
		return nil, fmt.Errorf("no data found for token %s", address)
	}

	// Parse string fields to appropriate types
	buyTax, _ := strconv.ParseFloat(tokenData.BuyTax, 64)
	sellTax, _ := strconv.ParseFloat(tokenData.SellTax, 64)
	transferTax, _ := strconv.ParseFloat(tokenData.TransferTax, 64)
	creatorPercent, _ := strconv.ParseFloat(tokenData.CreatorPercent, 64)
	holderCount, _ := strconv.Atoi(tokenData.HolderCount)
	lpHolderCount, _ := strconv.Atoi(tokenData.LPHolderCount)

	// Calculate top 10 holder concentration
	var top10Concentration float64
	if len(tokenData.Holders) > 0 {
		for _, holder := range tokenData.Holders {
			percent, _ := strconv.ParseFloat(holder.Percent, 64)
			top10Concentration += percent
		}
		top10Concentration *= 100 // Convert to percentage
	}

	// Determine if owner exists (not renounced)
	hasOwner := tokenData.OwnerAddress != "" &&
		tokenData.OwnerAddress != "0x0000000000000000000000000000000000000000"

	// Extract only the fields we need
	data := &GoPlusData{
		BuyTax:              buyTax,
		SellTax:             sellTax,
		TransferTax:         transferTax,
		CannotBuy:           tokenData.CannotBuy == "1",
		CannotSellAll:       tokenData.CannotSellAll == "1",
		CreatorPercent:      creatorPercent,
		HoneypotWithCreator: tokenData.HoneypotWithCreator == "1",
		HolderCount:         holderCount,
		Top10Concentration:  top10Concentration,
		IsProxy:             tokenData.IsProxy == "1",
		IsOpenSource:        tokenData.IsOpenSource == "1",
		HasOwner:            hasOwner,
		LPHolderCount:       lpHolderCount,
	}

	return data, nil
}
