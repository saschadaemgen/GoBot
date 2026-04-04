package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration values for GoBot.
type Config struct {
	// Version is the application version (set at build time).
	Version string

	// LogLevel controls log verbosity: debug, info, warn, error.
	LogLevel string

	// WSSPort is the WebSocket Secure listen port for GoKey connections.
	WSSPort int

	// Standalone enables standalone mode (local decryption, no GoKey).
	Standalone bool
}

// Default values matching the wire protocol specification.
const (
	defaultLogLevel   = "info"
	defaultWSSPort    = 6000
	defaultStandalone = true // standalone during Season 2 development
)

// Load reads configuration from environment variables with sensible
// defaults. Environment variables use the GOBOT_ prefix.
//
// Supported variables:
//   - GOBOT_LOG_LEVEL: debug, info, warn, error (default: info)
//   - GOBOT_WSS_PORT: WSS listen port (default: 6000)
//   - GOBOT_STANDALONE: enable standalone mode (default: true)
func Load() (*Config, error) {
	cfg := &Config{
		Version:    version(),
		LogLevel:   envOrDefault("GOBOT_LOG_LEVEL", defaultLogLevel),
		Standalone: envOrDefaultBool("GOBOT_STANDALONE", defaultStandalone),
	}

	port, err := envOrDefaultInt("GOBOT_WSS_PORT", defaultWSSPort)
	if err != nil {
		return nil, fmt.Errorf("invalid GOBOT_WSS_PORT: %w", err)
	}
	cfg.WSSPort = port

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("invalid log level %q: must be debug, info, warn, or error", c.LogLevel)
	}

	if c.WSSPort < 1 || c.WSSPort > 65535 {
		return fmt.Errorf("invalid WSS port %d: must be 1-65535", c.WSSPort)
	}

	return nil
}

// version returns the build version. Injected via ldflags at build time:
//
//	go build -ldflags "-X github.com/saschadaemgen/gobot/internal/config.buildVersion=1.0.0"
//
// Falls back to "dev" if not set.
var buildVersion string

func version() string {
	if buildVersion != "" {
		return buildVersion
	}
	return "dev"
}

func envOrDefault(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	return strconv.Atoi(v)
}

func envOrDefaultBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
