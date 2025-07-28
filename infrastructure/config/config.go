package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type SqliteConfig struct {
	Uri string `mapstructure:"uri"`
}

type Config struct {
	Debug  bool          `mapstructure:"debug"`
	Domain string        `mapstructure:"domain"`
	Port   int           `mapstructure:"port"`
	Tls    bool          `mapstructure:"tls"`
	Sqlite *SqliteConfig `mapstructure:"sqlite"`
}

// NewConfig creates a new Config instance by reading the configuration file.
// It expects the config file to be in YAML format and will parse it accordingly.
func NewConfig(configFile string) (*Config, error) {
	// split config file path in path and filename
	var configPath []string

	if strings.Contains(configFile, "/") {
		configPath = strings.Split(configFile, "/")
	} else {
		// if no path is given, set empty path. means that config is in "."
		configPath = []string{"./" + configFile}
	}

	// remove extension from filename
	fileName := strings.Split(configPath[len(configPath)-1], ".")[0]
	path := strings.Join(configPath[:len(configPath)-1], "/")

	viper.SetConfigName(fileName)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(path)

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// defaults
	viper.SetDefault("debug", false)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	if err := verifyConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

func verifyConfig(cfg *Config) error {
	if cfg.Domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if strings.Contains(cfg.Domain, "http") {
		return fmt.Errorf("domain musn't include the transport protocol")
	}
	if strings.Contains(cfg.Domain, ":") {
		return fmt.Errorf("domain mustn't include a port")
	}
	if strings.HasSuffix(cfg.Domain, "/") {
		cfg.Domain = strings.TrimRight(cfg.Domain, "/")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// verify database config
	if cfg.Sqlite != nil {
		if cfg.Sqlite.Uri == "" {
			return fmt.Errorf("missing sqlite uri")
		}
	} else {
		// we don't support any other databases just yet
		return fmt.Errorf("no database configured")
	}

	return nil
}
