package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	App App `mapstructure:"app"`
}

type App struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
}

func NewConfig(path string) *Config {
	if path == "" {
		panic(fmt.Errorf("config path is empty"))
	}
	filename := filepath.Join(path, "config.yaml")
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Errorf("failed to read config file: %w", err))
	}
	data = []byte(os.ExpandEnv(string(data)))
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	cfg := &Config{}
	if err := v.ReadConfig(bytes.NewBuffer(data)); err != nil {
		panic(fmt.Errorf("failed to read config: %w", err))
	}
	if err := v.Unmarshal(cfg); err != nil {
		panic(fmt.Errorf("failed to unmarshal config: %w", err))
	}
	return cfg
}
