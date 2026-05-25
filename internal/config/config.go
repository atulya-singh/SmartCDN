package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            int
	MinioEndpoint   string
	MinioAccessKey  string
	MinioSecretKey  string
	MinioSecure     bool
	MinioBucket     string
	RedisAddr       string
	RedisPassword   string
	CacheTTLSeconds int
	LogLevel        string
	UploadAPIKey    string
	RateLimitRPS    int
	RateLimitBurst  int
}

func Load() *Config {
	return &Config{
		Port:            getEnvInt("PORT", 8080),
		MinioEndpoint:   getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:  getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:  getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioSecure:     getEnvBool("MINIO_SECURE", false),
		MinioBucket:     getEnv("MINIO_BUCKET", "smartcdn-originals"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		CacheTTLSeconds: getEnvInt("CACHE_TTL_SECONDS", 86400),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		UploadAPIKey:    getEnv("UPLOAD_API_KEY", ""),
		RateLimitRPS:    getEnvInt("RATE_LIMIT_RPS", 100),
		RateLimitBurst:  getEnvInt("RATE_LIMIT_BURST", 200),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
