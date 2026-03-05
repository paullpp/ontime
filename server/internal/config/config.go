package config

import "github.com/caarlos0/env/v11"

type Config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`
	JWTSecret   string `env:"JWT_SECRET,required"`

	APNSKeyID    string `env:"APNS_KEY_ID"`
	APNSTeamID   string `env:"APNS_TEAM_ID"`
	APNSKeyFile  string `env:"APNS_KEY_FILE"`
	APNSBundleID string `env:"APNS_BUNDLE_ID"`

	GoogleMapsAPIKey string `env:"GOOGLE_MAPS_API_KEY"`

	Environment string `env:"ENVIRONMENT" envDefault:"development"`
	Port        int    `env:"PORT" envDefault:"8080"`

	// Mock flags — auto-enabled in non-production
	MockMaps bool `env:"MOCK_MAPS" envDefault:"false"`
	MockAPNS bool `env:"MOCK_APNS" envDefault:"false"`
	MockAuth bool `env:"MOCK_AUTH" envDefault:"false"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	if cfg.Environment != "production" {
		if !cfg.MockMaps {
			cfg.MockMaps = true
		}
		if !cfg.MockAPNS {
			cfg.MockAPNS = true
		}
	}
	return cfg, nil
}
