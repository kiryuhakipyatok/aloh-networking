package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

//go:embed config.yaml
var embeddedConfig []byte

type Config struct {
	App        App        `mapstructure:"app"`
	Signaling  Signaling  `mapstructure:"signaling"`
	Networking Networking `mapstructure:"networking"`
	Handler    Handler    `mapstructure:"handler"`
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
	NewSDPTimeout          time.Duration `mapstructure:"newSDPTimeout"`
	SendInStreamTimeout    time.Duration `mapstructure:"sendInStreamTimeout"`
	EstablishConnTimeout   time.Duration `mapstructure:"establishConnTimeout"`
	DatagramLogTargetCount uint          `mapstructure:"datagramLogTargetCount"`
}

type Handler struct {
	SendVideoTimeout   time.Duration `mapstructure:"sendVideoTimeout"`
	SendChatTimeout    time.Duration `mapstructure:"sendChatTimeout"`
	SendVoiceTimeout   time.Duration `mapstructure:"sendVoiceTimeout"`
	ConnectTimeout     time.Duration `mapstructure:"connectTimeout"`
	FetchOnlineTimeout time.Duration `mapstructure:"fetchOnlineTimeout"`
}

func NewConfig() *Config {

	if len(embeddedConfig) == 0 {
		panic("embedded config is empty")
	}
	data := []byte(os.ExpandEnv(string(embeddedConfig)))
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
