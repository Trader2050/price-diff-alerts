package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

const (
	cowQuotePath   = "/quote"
	zeroAddressHex = "0x0000000000000000000000000000000000000000"
)

var dec1e18 = decimal.NewFromInt(1_000_000_000_000_000_000)

// MarketOptions parameterise the CoW Protocol fetcher.
type MarketOptions struct {
	BaseURL      string
	PriceQuality string
	NotionalUSDE decimal.Decimal
	Timeout      time.Duration
	UserAgent    string
	SellToken    string
	BuyToken     string
}

// Market fetches quotes from CoW Protocol.
type Market struct {
	opts    MarketOptions
	logger  zerolog.Logger
	client  *http.Client
	baseURL string
}

// NewMarket constructs a market fetcher.
func NewMarket(opts MarketOptions, logger zerolog.Logger) *Market {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.cow.fi/mainnet/api/v1"
	}

	return &Market{
		opts:    opts,
		logger:  logger.With().Str("component", "market_fetcher").Logger(),
		client:  &http.Client{Timeout: timeout},
		baseURL: baseURL,
	}
}

// FetchMarket retrieves a CoW Protocol quote and returns sUSDe/USDe.
func (m *Market) FetchMarket(ctx context.Context) (decimal.Decimal, json.RawMessage, string, error) {
	if m.opts.NotionalUSDE.IsZero() {
		return decimal.Decimal{}, nil, "", errors.New("notional must be greater than zero")
	}
	if m.opts.SellToken == "" || m.opts.BuyToken == "" {
		return decimal.Decimal{}, nil, "", errors.New("sellToken and buyToken addresses required")
	}

	sellAtoms := m.opts.NotionalUSDE.Mul(dec1e18)
	sellAtoms = sellAtoms.Round(0)
	if sellAtoms.IsZero() {
		return decimal.Decimal{}, nil, "", errors.New("sell amount rounded to zero")
	}

	sellAmountBeforeFee := sellAtoms.StringFixed(0)

	reqPayload := quoteRequest{
		SellToken:           m.opts.SellToken,
		BuyToken:            m.opts.BuyToken,
		Kind:                "sell",
		From:                zeroAddressHex,
		AppData:             `{"version":"0.7.0","appCode":"usdewatcher","metadata":{}}`,
		PriceQuality:        m.opts.PriceQuality,
		SellAmountBeforeFee: sellAmountBeforeFee,
		ValidTo:             uint64(time.Now().Add(5 * time.Minute).Unix()),
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return decimal.Decimal{}, nil, "", err
	}

	endpoint := m.baseURL + cowQuotePath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return decimal.Decimal{}, nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if ua := strings.TrimSpace(m.opts.UserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	} else {
		req.Header.Set("User-Agent", "usdewatcher/1.0")
	}
	req.Header.Set("X-AppId", "usdewatcher")

	resp, err := m.client.Do(req)
	if err != nil {
		return decimal.Decimal{}, nil, "", err
	}
	defer resp.Body.Close()

	payloadBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return decimal.Decimal{}, nil, "", err
	}

	if resp.StatusCode != http.StatusOK {
		return decimal.Decimal{}, nil, "", parseHTTPError(resp.StatusCode, payloadBytes)
	}

	var quoteRes quoteResponse
	if err := json.Unmarshal(payloadBytes, &quoteRes); err != nil {
		return decimal.Decimal{}, nil, "", err
	}

	buyAtoms, err := decimal.NewFromString(quoteRes.Quote.BuyAmount)
	if err != nil {
		return decimal.Decimal{}, nil, "", fmt.Errorf("parse buy amount: %w", err)
	}

	if buyAtoms.IsZero() {
		return decimal.Decimal{}, nil, "", errors.New("buy amount returned zero")
	}

	rate := buyAtoms.Div(sellAtoms)

	quality := quoteRes.PriceQuality
	if quality == "" {
		quality = m.opts.PriceQuality
	}

	return rate, json.RawMessage(payloadBytes), quality, nil
}

type quoteRequest struct {
	SellToken           string `json:"sellToken"`
	BuyToken            string `json:"buyToken"`
	Kind                string `json:"kind"`
	From                string `json:"from"`
	AppData             string `json:"appData"`
	PriceQuality        string `json:"priceQuality,omitempty"`
	SellAmountBeforeFee string `json:"sellAmountBeforeFee"`
	ValidTo             uint64 `json:"validTo"`
}

type quoteResponse struct {
	Quote struct {
		SellAmount string `json:"sellAmount"`
		BuyAmount  string `json:"buyAmount"`
		FeeAmount  string `json:"feeAmount"`
		SellToken  string `json:"sellToken"`
		BuyToken   string `json:"buyToken"`
	} `json:"quote"`
	PriceQuality string `json:"priceQuality"`
}

type errorResponse struct {
	ErrorType   string `json:"errorType"`
	Description string `json:"description"`
	Message     string `json:"message"`
}

func parseHTTPError(status int, payload []byte) error {
	var apiErr errorResponse
	if err := json.Unmarshal(payload, &apiErr); err == nil {
		if apiErr.Description != "" {
			return fmt.Errorf("cow api error (%d): %s", status, apiErr.Description)
		}
		if apiErr.Message != "" {
			return fmt.Errorf("cow api error (%d): %s", status, apiErr.Message)
		}
		if apiErr.ErrorType != "" {
			return fmt.Errorf("cow api error (%d): %s", status, apiErr.ErrorType)
		}
	}
	if len(payload) > 0 {
		return fmt.Errorf("cow api error (%d): %s", status, strings.TrimSpace(string(payload)))
	}
	return fmt.Errorf("cow api error (%d)", status)
}

var _ MarketRateFetcher = (*Market)(nil)
