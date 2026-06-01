package alohnetwork

import (
	"github.com/kiryuhakipyatok/aloh-networking/config"
)

type (
	Config    = config.Config
	App       = config.App
	Signaling = config.Signaling
	Networking = config.Networking
	Handler = config.Handler
)

// // func NewConfig() *Config {

// // 	if len(embeddedConfig) == 0 {
// // 		panic("embedded config is empty")
// // 	}
// // 	data := []byte(os.ExpandEnv(string(embeddedConfig)))
// // 	v := viper.New()
// // 	v.SetConfigName("config")
// // 	v.SetConfigType("yaml")
// // 	cfg := &Config{}
// // 	if err := v.ReadConfig(bytes.NewBuffer(data)); err != nil {
// // 		panic(fmt.Errorf("failed to read config: %w", err))
// // 	}
// // 	if err := v.Unmarshal(cfg); err != nil {
// // 		panic(fmt.Errorf("failed to unmarshal config: %w", err))
// // 	}
// // 	return cfg
// // }
