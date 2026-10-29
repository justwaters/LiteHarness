package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds LiteHarness configuration.
type Config struct {
	Provider    string // "openai" or "gemini"
	BaseURL     string
	Model       string
	APIKey      string
	SystemPrompt string
}

const defaultConfigFile = ".liteharnessrc"

// LoadConfig reads configuration from env vars and ~/.liteharnessrc.
// Env vars take precedence over the config file.
func LoadConfig() *Config {
	cfg := &Config{
		Provider:    "openai",
		BaseURL:     "https://api.openai.com/v1",
		Model:       "gpt-3.5-turbo",
		SystemPrompt: "You are a helpful coding assistant.",
	}

	// Load from config file if it exists
	home, _ := os.UserHomeDir()
	if home != "" {
		path := home + "/" + defaultConfigFile
		if f, err := os.Open(path); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					switch key {
					case "provider":
						cfg.Provider = val
					case "base_url":
						cfg.BaseURL = val
					case "model":
						cfg.Model = val
					case "api_key":
						cfg.APIKey = val
					case "system_prompt":
						cfg.SystemPrompt = val
					}
				}
			}
			f.Close()
		}
	}

	// Env vars override file
	if v := os.Getenv("LITEHARNESS_PROVIDER"); v != "" {
		cfg.Provider = v
	}
	if v := os.Getenv("LITEHARNESS_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}
	if v := os.Getenv("LITEHARNESS_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("LITEHARNESS_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("LITEHARNESS_SYSTEM_PROMPT"); v != "" {
		cfg.SystemPrompt = v
	}

	return cfg
}

// SaveConfig writes the current config to ~/.liteharnessrc.
func SaveConfig(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := home + "/" + defaultConfigFile
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "provider=%s\n", cfg.Provider)
	fmt.Fprintf(f, "base_url=%s\n", cfg.BaseURL)
	fmt.Fprintf(f, "model=%s\n", cfg.Model)
	fmt.Fprintf(f, "api_key=%s\n", cfg.APIKey)
	fmt.Fprintf(f, "system_prompt=%s\n", cfg.SystemPrompt)
	return nil
}
