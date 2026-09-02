package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Sitemap, UserAgent, StatePath          string
	Delay                                  time.Duration
	Endpoint, AccessKey, SecretKey, Bucket string
	UseTLS                                 bool
}

func Load() (Config, error) {
	delay, err := time.ParseDuration(value("CONTENT_CRAWL_DELAY", "1500ms"))
	if err != nil {
		return Config{}, fmt.Errorf("CONTENT_CRAWL_DELAY: %w", err)
	}
	tls, err := strconv.ParseBool(value("CONTENT_S3_USE_TLS", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("CONTENT_S3_USE_TLS: %w", err)
	}
	c := Config{
		Sitemap:   value("CONTENT_SOURCE_SITEMAP", "https://learningenglish.voanews.com/sitemap.xml"),
		UserAgent: value("CONTENT_USER_AGENT", "VOALearningContentLibrary/0.1 (+https://example.com/contact)"),
		StatePath: value("CONTENT_STATE_PATH", "data/state.db"), Delay: delay,
		Endpoint: value("CONTENT_S3_ENDPOINT", "127.0.0.1:9002"), AccessKey: value("CONTENT_S3_ACCESS_KEY", "minioadmin"),
		SecretKey: value("CONTENT_S3_SECRET_KEY", "minioadmin"), Bucket: value("CONTENT_S3_BUCKET", "voa-content-library"), UseTLS: tls,
	}
	return c, nil
}

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
