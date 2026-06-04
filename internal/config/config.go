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
	AccrualWorkers       int    // ACCRUAL_WORKERS, number of concurrent accrual sync workers
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
	} else if env, ok := os.LookupEnv("RUN_ADDRESS"); ok && env != "" {
		cfg.ServerAddress = env
	} else {
		cfg.ServerAddress = defaultServerAddress
	}

	// DatabaseURI: flag > DATABASE_URI env > empty
	if dSet {
		cfg.DatabaseURI = *flagDatabaseURI
	} else if env, ok := os.LookupEnv("DATABASE_URI"); ok && env != "" {
		cfg.DatabaseURI = env
	} else {
		cfg.DatabaseURI = "" // Will cause error if not set
	}

	// AccrualSystemAddress: flag > ACCRUAL_SYSTEM_ADDRESS env > empty
	if rSet {
		cfg.AccrualSystemAddress = *flagAccrualAddress
	} else if env, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok && env != "" {
		cfg.AccrualSystemAddress = env
	} else {
		cfg.AccrualSystemAddress = ""
	}

	// AuthSecretKey: flag > AUTH_SECRET_KEY env > default
	if kSet {
		cfg.AuthSecretKey = *flagAuthSecretKey
	} else if env, ok := os.LookupEnv("AUTH_SECRET_KEY"); ok && env != "" {
		cfg.AuthSecretKey = env
	} else {
		cfg.AuthSecretKey = defaultAuthSecretKey
	}

	// RunEnv: flag > RUN_ENV env > default.
	// Для RUN_ENV пустая строка — валидное значение, нормализуется в "development" ниже.
	if eSet {
		cfg.RunEnv = *flagRunEnv
	} else if env, ok := os.LookupEnv("RUN_ENV"); ok {
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
	if envVal, ok := os.LookupEnv("ACCRUAL_INTERVAL_SEC"); ok && envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil && val > 0 {
			cfg.AccrualInterval = val
		}
	}

	// AccrualWorkers: default 5, overridable via ACCRUAL_WORKERS env.
	// 5 — компромисс между latency (параллельные HTTP-запросы) и нагрузкой
	// на сетевой стек / fd. accrual сам ограничивает RPS через 429 + Retry-After,
	// поэтому больше 5-10 воркеров в нашем масштабе не даст выигрыша.
	cfg.AccrualWorkers = 5
	if envVal, ok := os.LookupEnv("ACCRUAL_WORKERS"); ok && envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil && val > 0 {
			cfg.AccrualWorkers = val
		}
	}

	return cfg
}

// IsProduction returns true if RunEnv is set to production.
func (c *Config) IsProduction() bool {
	return c.RunEnv == "production"
}
