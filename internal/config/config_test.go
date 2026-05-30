package config

import (
	"os"
	"testing"
)

func TestConfig_Load_Defaults(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("RUN_ADDRESS")
	os.Unsetenv("DATABASE_URI")
	os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")
	os.Unsetenv("AUTH_SECRET_KEY")
	os.Unsetenv("RUN_ENV")

	cfg := Load()

	if cfg.ServerAddress != ":8080" {
		t.Errorf("ServerAddress = %q, want %q", cfg.ServerAddress, ":8080")
	}
	if cfg.DatabaseURI != "" {
		t.Errorf("DatabaseURI = %q, want empty", cfg.DatabaseURI)
	}
	if cfg.AccrualSystemAddress != "" {
		t.Errorf("AccrualSystemAddress = %q, want empty", cfg.AccrualSystemAddress)
	}
	if cfg.AuthSecretKey != "default-secret" {
		t.Errorf("AuthSecretKey = %q, want %q", cfg.AuthSecretKey, "default-secret")
	}
	if cfg.RunEnv != "development" {
		t.Errorf("RunEnv = %q, want %q", cfg.RunEnv, "development")
	}
}

func TestConfig_Load_FromEnvironment(t *testing.T) {
	// Clear environment variables first
	os.Unsetenv("RUN_ADDRESS")
	os.Unsetenv("DATABASE_URI")
	os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")
	os.Unsetenv("AUTH_SECRET_KEY")
	os.Unsetenv("RUN_ENV")

	os.Setenv("RUN_ADDRESS", "localhost:9090")
	os.Setenv("DATABASE_URI", "postgres://localhost:5432/test")
	os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://localhost:8081")
	os.Setenv("AUTH_SECRET_KEY", "my-secret-key")
	os.Setenv("RUN_ENV", "production")

	defer func() {
		os.Unsetenv("RUN_ADDRESS")
		os.Unsetenv("DATABASE_URI")
		os.Unsetenv("ACCRUAL_SYSTEM_ADDRESS")
		os.Unsetenv("AUTH_SECRET_KEY")
		os.Unsetenv("RUN_ENV")
	}()

	cfg := Load()

	if cfg.ServerAddress != "localhost:9090" {
		t.Errorf("ServerAddress = %q, want %q", cfg.ServerAddress, "localhost:9090")
	}
	if cfg.DatabaseURI != "postgres://localhost:5432/test" {
		t.Errorf("DatabaseURI = %q, want %q", cfg.DatabaseURI, "postgres://localhost:5432/test")
	}
	if cfg.AccrualSystemAddress != "http://localhost:8081" {
		t.Errorf("AccrualSystemAddress = %q, want %q", cfg.AccrualSystemAddress, "http://localhost:8081")
	}
	if cfg.AuthSecretKey != "my-secret-key" {
		t.Errorf("AuthSecretKey = %q, want %q", cfg.AuthSecretKey, "my-secret-key")
	}
	if cfg.RunEnv != "production" {
		t.Errorf("RunEnv = %q, want %q", cfg.RunEnv, "production")
	}
}

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name   string
		runEnv string
		want   bool
	}{
		{"production", "production", true},
		{"development", "development", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{RunEnv: tt.runEnv}
			if got := cfg.IsProduction(); got != tt.want {
				t.Errorf("IsProduction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_RunEnvNormalization(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wanted string
	}{
		{"lowercase", "production", "production"},
		{"uppercase", "PRODUCTION", "production"},
		{"mixed case", "Production", "production"},
		{"invalid becomes development", "invalid", "development"},
		{"empty becomes development", "", "development"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("RUN_ENV")
			os.Setenv("RUN_ENV", tt.input)
			defer os.Unsetenv("RUN_ENV")

			cfg := Load()
			if cfg.RunEnv != tt.wanted {
				t.Errorf("RunEnv = %q, want %q", cfg.RunEnv, tt.wanted)
			}
		})
	}
}
