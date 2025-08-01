package dynamicconfigservice

import (
	"fmt"
	"github.com/go-logr/logr"
	"github.com/joho/godotenv"
	"os"
	"strconv"
	"time"
)

type DataSyncControllerConfig struct {
	Concurrency          int
	RetryLimit           int
	RetryBackoffDuration time.Duration
	MaxSyncDuration      time.Duration
}

const (
	defaultConcurrency     = 10
	defaultRetryLimit      = 2
	defaultBackoffDuration = "10s"
	defaultMaxSyncDuration = "1h"
)

func GetDSControllerConfig(log logr.Logger) DataSyncControllerConfig {
	// Load a .env file to get these values if one exists
	godotenv.Load()

	concurrency, err := getEnvAsInt("CONCURRENCY", defaultConcurrency)
	if err != nil {
		msg := fmt.Sprintf("Invalid value for CONCURRENCY, using default: %d. Error: %v", defaultConcurrency, err)
		log.Info(msg)
	}

	retryLimit, err := getEnvAsInt("RETRY_LIMIT", defaultRetryLimit)
	if err != nil {
		msg := fmt.Sprintf("Invalid value for RETRY_LIMIT, using default: %d. Error: %v", defaultRetryLimit, err)
		log.Info(msg)
	}

	retryBackoffDuration, err := getEnvAsDuration("RETRY_BACKOFF_DURATION", defaultBackoffDuration)
	if err != nil {
		msg := fmt.Sprintf("Invalid value for RETRY_BACKOFF_DURATION, using default: %s. Error: %v", defaultBackoffDuration, err)
		log.Info(msg)
	}

	maxSyncDuration, err := getEnvAsDuration("MAX_SYNC_DURATION", defaultMaxSyncDuration)
	if err != nil {
		msg := fmt.Sprintf("Invalid value for MAX_SYNC_DURATION, using default: %s. Error: %v", defaultMaxSyncDuration, err)
		log.Info(msg)
	}

	return DataSyncControllerConfig{
		Concurrency:          concurrency,
		RetryLimit:           retryLimit,
		RetryBackoffDuration: retryBackoffDuration,
		MaxSyncDuration:      maxSyncDuration,
	}
}

func getEnvAsInt(name string, fallback int) (int, error) {
	if valueStr, ok := os.LookupEnv(name); ok {
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			return fallback, err
		}
		return value, nil
	}
	return fallback, nil
}

func getEnvAsDuration(name string, fallback string) (time.Duration, error) {
	if valueStr, ok := os.LookupEnv(name); ok {
		value, err := time.ParseDuration(valueStr)
		if err != nil {
			return time.ParseDuration(fallback)
		}
		return value, nil
	}
	return time.ParseDuration(fallback)
}
