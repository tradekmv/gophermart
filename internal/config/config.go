// Package config handles application configuration from flags and environment variables.
package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Config holds all application configuration.
type Config struct {
	ServerAddress        string // -a, RUN_ADDRESS, default ":8080"
	DatabaseURI          string // -d, DATABASE_URI
	AccrualSystemAddress string // -r, ACCRUAL_SYSTEM_ADDRESS
	AuthSecretKey        string // AUTH_SECRET_KEY, default "default-secret"
	RunEnv               string // RUN_ENV, "production" enables Secure cookies
	AccrualInterval      int    // ACCRUAL_INTERVAL_SEC, interval for accrual sync in seconds
}

const (
	defaultServerAddress = ":8080"
	defaultAuthSecretKey = "default-secret"
	defaultRunEnv        = "development"
)

var (
	initFlags          sync.Once
	flagServerAddress  *string
	flagDatabaseURI    *string
	flagAccrualAddress *string
	flagAuthSecretKey  *string
	flagRunEnv         *string
)

// Load parses flags and environment variables to build Config.
// Priority: flag > env > default value
func Load() *Config {
	initFlags.Do(func() {
		flagServerAddress = flag.String("a", defaultServerAddress, "server address")
		flagDatabaseURI = flag.String("d", "", "database URI")
		flagAccrualAddress = flag.String("r", "", "accrual system address")
		flagAuthSecretKey = flag.String("k", "", "authentication secret key")
		flagRunEnv = flag.String("e", "", "run environment (production/development)")
	})

	flag.Parse()

	// Track which flags were explicitly set on command line
	var aSet, dSet, rSet, kSet, eSet bool
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			aSet = true
		case "d":
			dSet = true
		case "r":
			rSet = true
		case "k":
			kSet = true
		case "e":
			eSet = true
		}
	})

	cfg := &Config{}

	// ServerAddress: flag > RUN_ADDRESS env > default
	if aSet {
		cfg.ServerAddress = *flagServerAddress
	} else if env := os.Getenv("RUN_ADDRESS"); env != "" {
		cfg.ServerAddress = env
	} else {
		cfg.ServerAddress = defaultServerAddress
	}

	// DatabaseURI: flag > DATABASE_URI env > empty
	if dSet {
		cfg.DatabaseURI = *flagDatabaseURI
	} else if env := os.Getenv("DATABASE_URI"); env != "" {
		cfg.DatabaseURI = env
	} else {
		cfg.DatabaseURI = "" // Will cause error if not set
	}

	// AccrualSystemAddress: flag > ACCRUAL_SYSTEM_ADDRESS env > empty
	if rSet {
		cfg.AccrualSystemAddress = *flagAccrualAddress
	} else if env := os.Getenv("ACCRUAL_SYSTEM_ADDRESS"); env != "" {
		cfg.AccrualSystemAddress = env
	} else {
		cfg.AccrualSystemAddress = ""
	}

	// AuthSecretKey: flag > AUTH_SECRET_KEY env > default
	if kSet {
		cfg.AuthSecretKey = *flagAuthSecretKey
	} else if env := os.Getenv("AUTH_SECRET_KEY"); env != "" {
		cfg.AuthSecretKey = env
	} else {
		cfg.AuthSecretKey = defaultAuthSecretKey
	}

	// RunEnv: flag > RUN_ENV env > default
	if eSet {
		cfg.RunEnv = *flagRunEnv
	} else if env := os.Getenv("RUN_ENV"); env != "" {
		cfg.RunEnv = env
	} else {
		cfg.RunEnv = defaultRunEnv
	}

	// Normalize RunEnv value
	cfg.RunEnv = strings.ToLower(cfg.RunEnv)
	if cfg.RunEnv != "production" && cfg.RunEnv != "development" {
		cfg.RunEnv = defaultRunEnv
	}

	// AccrualInterval: default 1 second, overridable via ACCRUAL_INTERVAL_SEC env
	cfg.AccrualInterval = 1
	if envVal := os.Getenv("ACCRUAL_INTERVAL_SEC"); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil && val > 0 {
			cfg.AccrualInterval = val
		}
	}

	return cfg
}

// IsProduction returns true if RunEnv is set to production.
func (c *Config) IsProduction() bool {
	return c.RunEnv == "production"
}
