package config

import (
	"fmt"
	"strings"
	"time"

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

type MediaConfig struct {
	StorageDir      string        `mapstructure:"storage_dir"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	UnattachedTTL   time.Duration `mapstructure:"unattached_ttl"`
}

type OAuthConfig struct {
	AllowPasswordGrant bool `mapstructure:"allow_password_grant"`
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
	Media       MediaConfig       `mapstructure:"media"`
	OAuth       OAuthConfig       `mapstructure:"oauth"`
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
	viper.SetDefault("web.cors.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	viper.SetDefault("web.cors.allowed_headers", []string{"Authorization", "Content-Type"})
	viper.SetDefault("web.cors.allow_credentials", false)
	viper.SetDefault("media.storage_dir", "./media")
	viper.SetDefault("media.cleanup_interval", "1h")
	viper.SetDefault("media.unattached_ttl", "24h")
	viper.SetDefault("oauth.allow_password_grant", false)

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

// verifyConfig validates cross-cutting infrastructure settings before wiring
// adapters. Keep it as orchestration; detailed rules live in focused helpers.
func verifyConfig(cfg *Config) error {
	if err := verifyDomainConfig(cfg); err != nil {
		return err
	}

	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if cfg.ActivityPub.BodyLimitBytes <= 0 {
		return fmt.Errorf("activitypub.body_limit_bytes must be greater than 0")
	}

	if err := verifyRemoteURLExceptions(cfg.ActivityPub.RemoteURLExceptions); err != nil {
		return err
	}
	if err := verifyCORSConfig(cfg.Web.CORS); err != nil {
		return err
	}

	if err := verifyMediaConfig(cfg.Media); err != nil {
		return err
	}
	return verifyDatabaseConfig(cfg)
}

// verifyDomainConfig normalizes host-facing values used to construct actor IDs,
// WebFinger URLs, OAuth redirects, and media URLs.
func verifyDomainConfig(cfg *Config) error {
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

	if cfg.PublicHost == "" {
		return nil
	}
	if !strings.HasPrefix(cfg.PublicHost, "http://") && !strings.HasPrefix(cfg.PublicHost, "https://") {
		return fmt.Errorf("public_host must include http:// or https://")
	}

	cfg.PublicHost = strings.TrimRight(cfg.PublicHost, "/")
	return nil
}

// verifyMediaConfig guards filesystem-backed media storage settings. The media
// adapter assumes these values are usable once composition completes.
func verifyMediaConfig(cfg MediaConfig) error {
	if strings.TrimSpace(cfg.StorageDir) == "" {
		return fmt.Errorf("media.storage_dir cannot be empty")
	}
	if cfg.CleanupInterval <= 0 {
		return fmt.Errorf("media.cleanup_interval must be greater than 0")
	}
	if cfg.UnattachedTTL <= 0 {
		return fmt.Errorf("media.unattached_ttl must be greater than 0")
	}
	return nil
}

// verifyDatabaseConfig keeps the current composition root explicit: SQLite is
// the only supported production database adapter today.
func verifyDatabaseConfig(cfg *Config) error {
	if cfg.Sqlite == nil {
		// we don't support any other databases just yet
		return fmt.Errorf("no database configured")
	}

	if cfg.Sqlite.Uri == "" {
		return fmt.Errorf("missing sqlite uri")
	}
	return nil
}
