package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
)

// Notification 封装告警上下文。
type Notification struct {
	Bucket        time.Time
	OfficialRate  decimal.Decimal
	MarketRate    decimal.Decimal
	DeviationPct  decimal.Decimal
	ThresholdPct  decimal.Decimal
	Direction     string
	Channels      []string
	NotionalUSDE  decimal.Decimal
	AdditionalMsg string
}

// Notifier 定义告警输送接口。
type Notifier interface {
	Notify(ctx context.Context, notification Notification) error
}

// TelegramNotifier 通过 Telegram Bot API 推送消息。
type TelegramNotifier struct {
	botToken string
	chatID   string
	baseURL  string
	client   *http.Client
	logger   zerolog.Logger
}

// NewTelegramNotifier 构造 Telegram 告警器。
func NewTelegramNotifier(botToken, chatID, baseURL string, timeout time.Duration, logger zerolog.Logger) *TelegramNotifier {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}

	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		baseURL:  strings.TrimRight(baseURL, "/"),
		client:   &http.Client{Timeout: timeout},
		logger:   logger.With().Str("component", "alert_telegram").Logger(),
	}
}

// Notify 调用 sendMessage API 推送文本。
func (n *TelegramNotifier) Notify(ctx context.Context, note Notification) error {
	payload := map[string]string{
		"chat_id": n.chatID,
		"text":    renderMessage(note),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", n.baseURL, n.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("send telegram request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram 响应码异常: %d", resp.StatusCode)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if !result.OK {
			return fmt.Errorf("telegram 返回 ok=false")
		}
	}

	n.logger.Info().Time("bucket", note.Bucket).
		Str("direction", note.Direction).
		Str("channels", strings.Join(note.Channels, ",")).
		Msg("告警已发送 (Telegram)")
	return nil
}

func renderMessage(note Notification) string {
	builder := strings.Builder{}
	builder.WriteString("[USDe-sUSDe Alert]\n")
	builder.WriteString(fmt.Sprintf("Bucket: %s UTC\n", note.Bucket.UTC().Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("Official: %s sUSDe/USDe\n", note.OfficialRate.StringFixed(3)))
	builder.WriteString(fmt.Sprintf("Market: %s sUSDe/USDe\n", note.MarketRate.StringFixed(3)))
	builder.WriteString(fmt.Sprintf("Deviation: %s%% (threshold %s%%)\n", note.DeviationPct.StringFixed(3), note.ThresholdPct.StringFixed(3)))
	builder.WriteString(fmt.Sprintf("Direction: %s\n", note.Direction))
	builder.WriteString(fmt.Sprintf("Notional: %s USDe\n", note.NotionalUSDE.String()))
	if len(note.Channels) > 0 {
		builder.WriteString(fmt.Sprintf("Channels: %s\n", strings.Join(note.Channels, ",")))
	}
	if note.AdditionalMsg != "" {
		builder.WriteString(note.AdditionalMsg)
	}
	return builder.String()
}

var _ Notifier = (*TelegramNotifier)(nil)
