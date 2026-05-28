package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type SqliteConfig struct {
	Uri string `mapstructure:"uri"`
}

type ActivityPubRemoteURLException struct {
	Host           string `mapstructure:"host"`
	AllowHTTP      bool   `mapstructure:"allow_http"`
	AllowPrivateIP bool   `mapstructure:"allow_private_ip"`
}

type ActivityPubConfig struct {
	BodyLimitBytes      int                             `mapstructure:"body_limit_bytes"`
	RemoteURLExceptions []ActivityPubRemoteURLException `mapstructure:"remote_url_exceptions"`
	DeliveryQueueSize   int                             `mapstructure:"delivery_queue_size"`
}

type CORSConfig struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

type WebConfig struct {
	CORS CORSConfig `mapstructure:"cors"`
}

type Config struct {
	Debug       bool              `mapstructure:"debug"`
	Domain      string            `mapstructure:"domain"`
	PublicHost  string            `mapstructure:"public_host"`
	Port        int               `mapstructure:"port"`
	Tls         bool              `mapstructure:"tls"`
	Sqlite      *SqliteConfig     `mapstructure:"sqlite"`
	ActivityPub ActivityPubConfig `mapstructure:"activitypub"`
	Web         WebConfig         `mapstructure:"web"`
}

func (c Config) Host() string {
	if c.PublicHost != "" {
		return strings.TrimRight(c.PublicHost, "/")
	}

	var host string

	if c.Tls {
		host = fmt.Sprintf("https://%s", c.Domain)
		if c.Port != 443 {
			host += fmt.Sprintf(":%d", c.Port)
		}
	} else {
		host = fmt.Sprintf("http://%s", c.Domain)
		if c.Port != 80 {
			host += fmt.Sprintf(":%d", c.Port)
		}
	}

	return host
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
	viper.SetDefault("activitypub.body_limit_bytes", 1<<20)
	viper.SetDefault("activitypub.delivery_queue_size", 128)
	viper.SetDefault("web.cors.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	viper.SetDefault("web.cors.allowed_headers", []string{"Authorization", "Content-Type"})
	viper.SetDefault("web.cors.allow_credentials", false)

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

func verifyRemoteURLExceptions(exceptions []ActivityPubRemoteURLException) error {
	seen := map[string]bool{}
	for _, exception := range exceptions {
		host := strings.TrimSpace(exception.Host)
		if host == "" {
			return fmt.Errorf("activitypub.remote_url_exceptions.host cannot be empty")
		}
		if strings.Contains(host, "://") || strings.Contains(host, "/") {
			return fmt.Errorf("activitypub.remote_url_exceptions.host must be a hostname, not a URL")
		}
		if host == "*" {
			return fmt.Errorf("activitypub.remote_url_exceptions.host must not be wildcard")
		}
		if seen[host] {
			return fmt.Errorf("duplicate activitypub.remote_url_exceptions host %q", host)
		}
		seen[host] = true
	}
	return nil
}

func verifyCORSConfig(cfg CORSConfig) error {
	for _, origin := range cfg.AllowedOrigins {
		if strings.TrimSpace(origin) == "" {
			return fmt.Errorf("web.cors.allowed_origins cannot contain empty origins")
		}
		if origin == "*" {
			return fmt.Errorf("web.cors.allowed_origins must not use wildcard origins")
		}
		if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
			return fmt.Errorf("web.cors.allowed_origins entries must include http:// or https://")
		}
		if strings.HasSuffix(origin, "/") {
			return fmt.Errorf("web.cors.allowed_origins entries must not end with a slash")
		}
	}
	if len(cfg.AllowedOrigins) > 0 {
		if len(cfg.AllowedMethods) == 0 {
			return fmt.Errorf("web.cors.allowed_methods cannot be empty when CORS is enabled")
		}
		if len(cfg.AllowedHeaders) == 0 {
			return fmt.Errorf("web.cors.allowed_headers cannot be empty when CORS is enabled")
		}
	}
	return nil
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
	if cfg.PublicHost != "" {
		if !strings.HasPrefix(cfg.PublicHost, "http://") && !strings.HasPrefix(cfg.PublicHost, "https://") {
			return fmt.Errorf("public_host must include http:// or https://")
		}
		cfg.PublicHost = strings.TrimRight(cfg.PublicHost, "/")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if cfg.ActivityPub.BodyLimitBytes <= 0 {
		return fmt.Errorf("activitypub.body_limit_bytes must be greater than 0")
	}
	if cfg.ActivityPub.DeliveryQueueSize <= 0 {
		return fmt.Errorf("activitypub.delivery_queue_size must be greater than 0")
	}
	if err := verifyRemoteURLExceptions(cfg.ActivityPub.RemoteURLExceptions); err != nil {
		return err
	}
	if err := verifyCORSConfig(cfg.Web.CORS); err != nil {
		return err
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
