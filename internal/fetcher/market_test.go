package fetcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestMarketFetchMissingTokens(t *testing.T) {
	m := NewMarket(MarketOptions{NotionalUSDE: decimal.NewFromInt(1)}, noopLogger())
	if _, _, _, err := m.FetchMarket(context.Background()); err == nil {
		t.Fatal("缺少 token 时应返回错误")
	}
}

func TestMarketFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"errorType": "bad"})
	}))
	defer srv.Close()

	m := NewMarket(MarketOptions{
		BaseURL:      srv.URL,
		PriceQuality: "optimal",
		NotionalUSDE: decimal.NewFromInt(1),
		Timeout:      time.Second,
		UserAgent:    "test",
		SellToken:    "0x1",
		BuyToken:     "0x2",
	}, noopLogger())

	if _, _, _, err := m.FetchMarket(context.Background()); err == nil {
		t.Fatal("HTTP 400 应返回错误")
	}
}

func TestMarketFetchSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"quote": map[string]string{
				"sellAmount": "1000000000000000000",
				"buyAmount":  "2000000000000000000",
				"feeAmount":  "0",
			},
			"priceQuality": "verified",
		})
	}))
	defer srv.Close()

	m := NewMarket(MarketOptions{
		BaseURL:      srv.URL,
		PriceQuality: "optimal",
		NotionalUSDE: decimal.NewFromInt(1),
		Timeout:      time.Second,
		UserAgent:    "test",
		SellToken:    "0x1",
		BuyToken:     "0x2",
	}, noopLogger())

	rate, _, quality, err := m.FetchMarket(context.Background())
	if err != nil {
		t.Fatalf("成功响应不应报错: %v", err)
	}
	if rate.Cmp(decimal.NewFromInt(2)) != 0 {
		t.Fatalf("期望汇率 2, 实际 %s", rate.String())
	}
	if quality != "verified" {
		t.Fatalf("应返回响应中的 priceQuality")
	}
}
