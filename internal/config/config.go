package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envFile       = ".env"
	tokensEnvFile = ".env.tokens"
)

type Config struct {
	HTTPAddress string
	Database    DatabaseConfig
	JobQuery    string
	Adzuna      AdzunaConfig
	Sync        SyncConfig
}

type DatabaseConfig struct {
	Path string
}

type AdzunaConfig struct {
	AppID          string
	APIKey         string
	Country        string
	MaxDaysOld     int
	MaxPages       int
	ResultsPerPage int
	Workplace      string
}

type SyncConfig struct {
	Interval time.Duration
}

func Load() (Config, error) {
	values, err := readEnvFile(envFile)
	if err != nil {
		return Config{}, err
	}
	if values["ADZUNA_APP_ID"] != "" || values["ADZUNA_API_KEY"] != "" {
		return Config{}, fmt.Errorf("move ADZUNA_APP_ID and ADZUNA_API_KEY from %s to %s", envFile, tokensEnvFile)
	}
	tokens, err := readEnvFile(tokensEnvFile)
	if err != nil {
		return Config{}, err
	}
	for key, value := range tokens {
		if _, exists := values[key]; exists {
			return Config{}, fmt.Errorf("%s must not be defined in both %s and %s", key, envFile, tokensEnvFile)
		}
		values[key] = value
	}
	for _, key := range []string{
		"HTTP_ADDRESS",
		"DATABASE_PATH",
		"ADZUNA_APP_ID",
		"ADZUNA_API_KEY",
		"ADZUNA_COUNTRY",
		"JOB_QUERY",
	} {
		if requiredString(values, key) == "" {
			return Config{}, fmt.Errorf("%s is required", key)
		}
	}

	maxDaysOld, err := requiredPositiveInt(values, "ADZUNA_MAX_DAYS_OLD")
	if err != nil {
		return Config{}, err
	}
	maxPages, err := requiredPositiveInt(values, "ADZUNA_MAX_PAGES")
	if err != nil {
		return Config{}, err
	}
	resultsPerPage, err := requiredPositiveInt(values, "ADZUNA_RESULTS_PER_PAGE")
	if err != nil {
		return Config{}, err
	}
	interval, err := requiredDuration(values, "SYNC_INTERVAL")
	if err != nil {
		return Config{}, err
	}
	workplace, err := requiredWorkplace(values)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress: requiredString(values, "HTTP_ADDRESS"),
		Database: DatabaseConfig{
			Path: requiredString(values, "DATABASE_PATH"),
		},
		JobQuery: requiredString(values, "JOB_QUERY"),
		Adzuna: AdzunaConfig{
			AppID:          requiredString(values, "ADZUNA_APP_ID"),
			APIKey:         requiredString(values, "ADZUNA_API_KEY"),
			Country:        requiredString(values, "ADZUNA_COUNTRY"),
			MaxDaysOld:     maxDaysOld,
			MaxPages:       maxPages,
			ResultsPerPage: resultsPerPage,
			Workplace:      workplace,
		},
		Sync: SyncConfig{
			Interval: interval,
		},
	}, nil
}

func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("invalid configuration line in %s", path)
		}
		if _, exists := values[key]; exists {
			return nil, fmt.Errorf("duplicate configuration key %s in %s", key, path)
		}
		values[key] = strings.Trim(strings.TrimSpace(value), "\"'")
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func requiredString(values map[string]string, key string) string {
	return strings.TrimSpace(values[key])
}

func requiredPositiveInt(values map[string]string, key string) (int, error) {
	value := requiredString(values, key)
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}

func requiredDuration(values map[string]string, key string) (time.Duration, error) {
	value := requiredString(values, key)
	if value == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", key)
	}
	return parsed, nil
}

func requiredWorkplace(values map[string]string) (string, error) {
	workplace := requiredString(values, "ADZUNA_WORKPLACE")
	switch workplace {
	case "any", "remote", "remote-hybrid":
		return workplace, nil
	default:
		return "", fmt.Errorf("ADZUNA_WORKPLACE must be any, remote, or remote-hybrid")
	}
}
