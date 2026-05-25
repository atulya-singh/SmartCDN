package config

import (
	"testing"
)

var allConfigKeys = []string{
	"PORT", "MINIO_ENDPOINT", "MINIO_ACCESS_KEY", "MINIO_SECRET_KEY",
	"MINIO_SECURE", "MINIO_BUCKET", "REDIS_ADDR", "REDIS_PASSWORD",
	"CACHE_TTL_SECONDS", "LOG_LEVEL",
}

func TestLoad_Defaults(t *testing.T) {
	for _, k := range allConfigKeys {
		t.Setenv(k, "")
	}

	cfg := Load()

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"Port", cfg.Port, 8080},
		{"MinioEndpoint", cfg.MinioEndpoint, "localhost:9000"},
		{"MinioAccessKey", cfg.MinioAccessKey, "minioadmin"},
		{"MinioSecretKey", cfg.MinioSecretKey, "minioadmin"},
		{"MinioSecure", cfg.MinioSecure, false},
		{"MinioBucket", cfg.MinioBucket, "smartcdn-originals"},
		{"RedisAddr", cfg.RedisAddr, "localhost:6379"},
		{"RedisPassword", cfg.RedisPassword, ""},
		{"CacheTTLSeconds", cfg.CacheTTLSeconds, 86400},
		{"LogLevel", cfg.LogLevel, "info"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9999")
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "mykey")
	t.Setenv("MINIO_SECRET_KEY", "mysecret")
	t.Setenv("MINIO_SECURE", "true")
	t.Setenv("MINIO_BUCKET", "my-bucket")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "hunter2")
	t.Setenv("CACHE_TTL_SECONDS", "3600")
	t.Setenv("LOG_LEVEL", "debug")

	cfg := Load()

	if cfg.Port != 9999 {
		t.Errorf("Port = %d, want 9999", cfg.Port)
	}
	if cfg.MinioEndpoint != "minio:9000" {
		t.Errorf("MinioEndpoint = %q, want minio:9000", cfg.MinioEndpoint)
	}
	if cfg.MinioAccessKey != "mykey" {
		t.Errorf("MinioAccessKey = %q, want mykey", cfg.MinioAccessKey)
	}
	if !cfg.MinioSecure {
		t.Error("MinioSecure = false, want true")
	}
	if cfg.MinioBucket != "my-bucket" {
		t.Errorf("MinioBucket = %q, want my-bucket", cfg.MinioBucket)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %q, want redis:6379", cfg.RedisAddr)
	}
	if cfg.RedisPassword != "hunter2" {
		t.Errorf("RedisPassword = %q, want hunter2", cfg.RedisPassword)
	}
	if cfg.CacheTTLSeconds != 3600 {
		t.Errorf("CacheTTLSeconds = %d, want 3600", cfg.CacheTTLSeconds)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", cfg.LogLevel)
	}
}

func TestLoad_InvalidIntFallsBackToDefault(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("CACHE_TTL_SECONDS", "abc")

	cfg := Load()

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080", cfg.Port)
	}
	if cfg.CacheTTLSeconds != 86400 {
		t.Errorf("CacheTTLSeconds = %d, want default 86400", cfg.CacheTTLSeconds)
	}
}

func TestLoad_InvalidBoolFallsBackToDefault(t *testing.T) {
	t.Setenv("MINIO_SECURE", "not-a-bool")

	cfg := Load()

	if cfg.MinioSecure != false {
		t.Errorf("MinioSecure = %v, want default false", cfg.MinioSecure)
	}
}

func TestLoad_BoolVariants(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"TRUE", true},
		{"FALSE", false},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			t.Setenv("MINIO_SECURE", tt.val)
			cfg := Load()
			if cfg.MinioSecure != tt.want {
				t.Errorf("MINIO_SECURE=%q → MinioSecure = %v, want %v", tt.val, cfg.MinioSecure, tt.want)
			}
		})
	}
}
