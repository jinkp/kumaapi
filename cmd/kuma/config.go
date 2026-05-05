package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	URL   string `mapstructure:"url" yaml:"url"`
	Token string `mapstructure:"token" yaml:"token"`
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".kuma", "config.yaml"), nil
}

func loadConfig() (Config, string, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return Config{}, "", err
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return Config{}, path, nil
		}
		if os.IsNotExist(err) {
			return Config{}, path, nil
		}
		return Config{}, path, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, path, fmt.Errorf("decode config: %w", err)
	}

	return cfg, path, nil
}

func saveConfig(cfg Config) error {
	path, err := defaultConfigPath()
	if err != nil {
		return err
	}

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.Set("url", cfg.URL)
	v.Set("token", cfg.Token)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
