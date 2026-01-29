package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App        App        `mapstructure:"app"`
	Signaling  Signaling  `mapstructure:"signaling"`
	Networking Networking `mapstructure:"networking"`
}

type App struct {
	Name           string `mapstructure:"name"`
	Version        string `mapstructure:"version"`
	Env            string `mapstructure:"env"`
	ReceiveSDPSize int    `mapstructure:"receiveSDPSize"`
	SendSDPSize    int    `mapstructure:"sendSDPSize"`
}

type Signaling struct {
	Address                string        `mapstructure:"address"`
	Port                   string        `mapstructure:"port"`
	IdleTimeout            time.Duration `mapstructure:"idleTimeout"`
	HandshakeTimeout       time.Duration `mapstructure:"handshakeTimeout"`
	KeepAlivePeriodTimeout time.Duration `mapstructure:"keepAlivePeriodTimeout"`
	CloseTimeout           time.Duration `mapstructure:"closeTimeout"`
	StartTimeout           time.Duration `mapstructure:"startTimeout"`
	NextProtos             []string      `mapstructure:"nextProtos"`
	MaxIncomingStreams     int64         `mapstructure:"maxIncomingStreams"`
	MaxIncomingUniStreams  int64         `mapstructure:"maxIncomingUniStreams"`
	RegTimeout             time.Duration `mapstructure:"regTimeout"`
}

type Networking struct {
	IdleTimeout            time.Duration `mapstructure:"idleTimeout"`
	HandshakeTimeout       time.Duration `mapstructure:"handshakeTimeout"`
	KeepAlivePeriodTimeout time.Duration `mapstructure:"keepAlivePeriodTimeout"`
	NextProtos             []string      `mapstructure:"nextProtos"`
	STUNHost               string        `mapstructure:"stunHost"`
	STUNPort               int           `mapstructure:"stunPort"`
	TURNHost               string        `mapstructure:"turnHost"`
	TURNPort               int           `mapstructure:"turnPort"`
	TURNUsername           string        `mapstructure:"turnUsername"`
	TURNPassword           string        `mapstructure:"turnPassword"`
	NewSDPTimeout          time.Duration `mapstructure:"newSDPTimeout"`
	SendInStreamTimeout    time.Duration `mapstructure:"sendInStreamTimeout"`
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
