package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env vars that might interfere.
	os.Unsetenv("GOBOT_LOG_LEVEL")
	os.Unsetenv("GOBOT_WSS_PORT")
	os.Unsetenv("GOBOT_STANDALONE")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}

	if cfg.WSSPort != 6000 {
		t.Errorf("WSSPort = %d, want %d", cfg.WSSPort, 6000)
	}

	if cfg.Standalone != true {
		t.Errorf("Standalone = %v, want %v", cfg.Standalone, true)
	}

	if cfg.Version != "dev" {
		t.Errorf("Version = %q, want %q", cfg.Version, "dev")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("GOBOT_LOG_LEVEL", "debug")
	t.Setenv("GOBOT_WSS_PORT", "7000")
	t.Setenv("GOBOT_STANDALONE", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}

	if cfg.WSSPort != 7000 {
		t.Errorf("WSSPort = %d, want %d", cfg.WSSPort, 7000)
	}

	if cfg.Standalone != false {
		t.Errorf("Standalone = %v, want %v", cfg.Standalone, false)
	}
}

func TestLoadInvalidLogLevel(t *testing.T) {
	t.Setenv("GOBOT_LOG_LEVEL", "verbose")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error for invalid log level")
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("GOBOT_WSS_PORT", "99999")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error for invalid port")
	}
}

func TestLoadNonNumericPort(t *testing.T) {
	t.Setenv("GOBOT_WSS_PORT", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error for non-numeric port")
	}
}
