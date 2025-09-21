package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	"price-diff-alerts/internal/logging"
)

// Config materialises application configuration.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	Logging   logging.Config  `mapstructure:"logging"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Scheduler SchedulerConfig `mapstructure:"scheduler"`
	Ethereum  EthereumConfig  `mapstructure:"ethereum"`
	Cow       CowConfig       `mapstructure:"cow"`
	Alerting  AlertingConfig  `mapstructure:"alerting"`
	Export    ExportConfig    `mapstructure:"export"`
}

// AppConfig general metadata.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
}

// DatabaseConfig encapsulates PostgreSQL connectivity.
type DatabaseConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MigrationsPath  string        `mapstructure:"migrations_path"`
}

// SchedulerConfig governs sampling cadence.
type SchedulerConfig struct {
	Interval        time.Duration `mapstructure:"interval"`
	AlignToBucket   bool          `mapstructure:"align_to_bucket"`
	AdvisoryLockKey int64         `mapstructure:"advisory_lock_key"`
	StartupDelay    time.Duration `mapstructure:"startup_delay"`
}

// EthereumConfig covers on-chain data access.
type EthereumConfig struct {
	RPCURL         string        `mapstructure:"rpc_url"`
	SUSDEAddress   string        `mapstructure:"susde_address"`
	USDEAddress    string        `mapstructure:"usde_address"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
}

// CowConfig captures CoW Protocol connectivity.
type CowConfig struct {
	BaseURL        string        `mapstructure:"base_url"`
	PriceQuality   string        `mapstructure:"price_quality"`
	NotionalUSDE   float64       `mapstructure:"notional_usde"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	UserAgent      string        `mapstructure:"user_agent"`
}

// AlertingConfig defines alert thresholds and routing.
type AlertingConfig struct {
	Enabled      bool           `mapstructure:"enabled"`
	ThresholdPct float64        `mapstructure:"threshold_pct"`
	Cooldown     time.Duration  `mapstructure:"cooldown"`
	Channels     []string       `mapstructure:"channels"`
	Telegram     TelegramConfig `mapstructure:"telegram"`
}

// TelegramConfig 描述 Telegram 告警参数。
type TelegramConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	ChatID   string `mapstructure:"chat_id"`
	APIBase  string `mapstructure:"api_base"`
}

// ExportConfig sets CLI export behaviour.
type ExportConfig struct {
	MaxDataPoints int `mapstructure:"max_data_points"`
}

// Load builds configuration from file, environment, and defaults.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("USDEWATCHER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}

	if err := readConfig(v); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, decodeHook()); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func readConfig(v *viper.Viper) error {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "usdewatcher")
	v.SetDefault("app.environment", "development")

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")

	v.SetDefault("scheduler.interval", "5m")
	v.SetDefault("scheduler.align_to_bucket", true)
	v.SetDefault("scheduler.advisory_lock_key", int64(0x75734445))
	v.SetDefault("scheduler.startup_delay", "0s")

	v.SetDefault("ethereum.request_timeout", "10s")

	v.SetDefault("cow.base_url", "https://api.cow.fi/mainnet/api/v1")
	v.SetDefault("cow.price_quality", "optimal")
	v.SetDefault("cow.notional_usde", 10000.0)
	v.SetDefault("cow.request_timeout", "10s")
	v.SetDefault("cow.user_agent", "usdewatcher/1.0")

	v.SetDefault("alerting.enabled", false)
	v.SetDefault("alerting.threshold_pct", 0.4)
	v.SetDefault("alerting.cooldown", "30m")
	v.SetDefault("alerting.channels", []string{"telegram"})
	v.SetDefault("alerting.telegram.enabled", false)
	v.SetDefault("alerting.telegram.api_base", "https://api.telegram.org")

	v.SetDefault("export.max_data_points", 100000)

	v.SetDefault("database.max_open_conns", 10)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "30m")
	v.SetDefault("database.migrations_path", "migrations")
}

func decodeHook() viper.DecoderConfigOption {
	return func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "mapstructure"
		dc.DecodeHook = mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		)
	}
}

// Validate performs basic sanity checks on the configuration values.
func (c *Config) Validate() error {
	if c.Export.MaxDataPoints <= 0 {
		return fmt.Errorf("export.max_data_points must be greater than zero")
	}
	if c.Scheduler.Interval <= 0 {
		return fmt.Errorf("scheduler.interval must be greater than zero")
	}
	if c.Cow.NotionalUSDE <= 0 {
		return fmt.Errorf("cow.notional_usde must be greater than zero")
	}
	if c.Alerting.ThresholdPct < 0 {
		return fmt.Errorf("alerting.threshold_pct cannot be negative")
	}
	if c.Alerting.Telegram.Enabled {
		if c.Alerting.Telegram.BotToken == "" {
			return fmt.Errorf("alerting.telegram.bot_token 必须配置")
		}
		if c.Alerting.Telegram.ChatID == "" {
			return fmt.Errorf("alerting.telegram.chat_id 必须配置")
		}
	}
	return nil
}

// ResolveMaxPoints returns either the CLI override or config default.
func (c *Config) ResolveMaxPoints(override int) int {
	if override > 0 {
		return override
	}
	return c.Export.MaxDataPoints
}
