package alerting

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

func TestTelegramNotifierSuccess(t *testing.T) {
	received := make(map[string]string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "sendMessage") {
			t.Fatalf("路径应包含 sendMessage, 实际 %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("解析请求体失败: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	notifier := NewTelegramNotifier("token", "chat", srv.URL, time.Second, testLogger())
	note := Notification{Bucket: time.Now(), OfficialRate: decimal.NewFromInt(1), MarketRate: decimal.NewFromInt(1), DeviationPct: decimal.NewFromInt(1), ThresholdPct: decimal.NewFromInt(1), NotionalUSDE: decimal.NewFromInt(1)}

	if err := notifier.Notify(context.Background(), note); err != nil {
		t.Fatalf("Telegram Notify 应成功: %v", err)
	}

	if received["chat_id"] != "chat" {
		t.Fatalf("chat_id 不正确: %#v", received)
	}
	if received["text"] == "" {
		t.Fatalf("text 应非空")
	}
}

func TestTelegramNotifierError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": false})
	}))
	defer srv.Close()

	notifier := NewTelegramNotifier("token", "chat", srv.URL, time.Second, testLogger())
	note := Notification{Bucket: time.Now(), OfficialRate: decimal.NewFromInt(1), MarketRate: decimal.NewFromInt(1), DeviationPct: decimal.NewFromInt(1), ThresholdPct: decimal.NewFromInt(1), NotionalUSDE: decimal.NewFromInt(1)}

	if err := notifier.Notify(context.Background(), note); err == nil {
		t.Fatal("ok=false 应报错")
	}
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}
