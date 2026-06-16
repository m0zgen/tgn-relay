package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen   string                 `yaml:"listen"`
	Telegram TelegramConfig         `yaml:"telegram"`
	Security SecurityConfig         `yaml:"security"`
	Groups   map[string]GroupConfig `yaml:"groups"`
}

type TelegramConfig struct {
	APIURL  string `yaml:"api_url"`
	Timeout string `yaml:"timeout"`
}

type SecurityConfig struct {
	RelayKeys         []string `yaml:"relay_keys"`
	AllowIPs          []string `yaml:"allow_ips"`
	DirectModeEnabled bool     `yaml:"direct_mode_enabled"`
	MaxTextBytes      int64    `yaml:"max_text_bytes"`
}

type GroupConfig struct {
	Token                 string `yaml:"token"`
	ChatID                string `yaml:"chat_id"`
	ParseMode             string `yaml:"parse_mode"`
	DisableWebPagePreview bool   `yaml:"disable_web_page_preview"`
	DisableNotification   bool   `yaml:"disable_notification"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	cfg.setDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) setDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.Telegram.APIURL == "" {
		c.Telegram.APIURL = "https://api.telegram.org"
	}
	if c.Telegram.Timeout == "" {
		c.Telegram.Timeout = "7s"
	}
	if c.Security.MaxTextBytes == 0 {
		c.Security.MaxTextBytes = 4096
	}
}

func (c *Config) Validate() error {
	if c.Listen == "" {
		return errors.New("listen is required")
	}
	if !strings.HasPrefix(c.Telegram.APIURL, "https://") {
		return errors.New("telegram.api_url must start with https://")
	}
	if _, err := time.ParseDuration(c.Telegram.Timeout); err != nil {
		return fmt.Errorf("invalid telegram.timeout: %w", err)
	}
	if len(c.Security.RelayKeys) == 0 {
		return errors.New("security.relay_keys must contain at least one key")
	}
	if c.Security.MaxTextBytes < 1 || c.Security.MaxTextBytes > 65536 {
		return errors.New("security.max_text_bytes must be between 1 and 65536")
	}
	for _, cidr := range c.Security.AllowIPs {
		if _, err := netip.ParsePrefix(cidr); err != nil {
			return fmt.Errorf("invalid allow_ips CIDR %q: %w", cidr, err)
		}
	}
	for name, g := range c.Groups {
		if name == "" {
			return errors.New("group name cannot be empty")
		}
		if g.Token == "" {
			return fmt.Errorf("group %q token is required", name)
		}
		if g.ChatID == "" {
			return fmt.Errorf("group %q chat_id is required", name)
		}
	}
	return nil
}

func (t TelegramConfig) TimeoutDuration() time.Duration {
	d, err := time.ParseDuration(t.Timeout)
	if err != nil {
		return 7 * time.Second
	}
	return d
}
