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
	AdzunaInterval   time.Duration
	RemotiveInterval time.Duration
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
	httpAddress, err := requiredString(values, "HTTP_ADDRESS")
	if err != nil {
		return Config{}, err
	}
	databasePath, err := requiredString(values, "DATABASE_PATH")
	if err != nil {
		return Config{}, err
	}
	adzunaAppID, err := requiredString(values, "ADZUNA_APP_ID")
	if err != nil {
		return Config{}, err
	}
	adzunaAPIKey, err := requiredString(values, "ADZUNA_API_KEY")
	if err != nil {
		return Config{}, err
	}
	adzunaCountry, err := requiredString(values, "ADZUNA_COUNTRY")
	if err != nil {
		return Config{}, err
	}
	jobQuery, err := requiredString(values, "JOB_QUERY")
	if err != nil {
		return Config{}, err
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
	adzunaInterval, err := requiredDuration(values, "ADZUNA_SYNC_INTERVAL")
	if err != nil {
		return Config{}, err
	}
	remotiveInterval, err := requiredDuration(values, "REMOTIVE_SYNC_INTERVAL")
	if err != nil {
		return Config{}, err
	}
	workplace, err := requiredWorkplace(values)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress: httpAddress,
		Database: DatabaseConfig{
			Path: databasePath,
		},
		JobQuery: jobQuery,
		Adzuna: AdzunaConfig{
			AppID:          adzunaAppID,
			APIKey:         adzunaAPIKey,
			Country:        adzunaCountry,
			MaxDaysOld:     maxDaysOld,
			MaxPages:       maxPages,
			ResultsPerPage: resultsPerPage,
			Workplace:      workplace,
		},
		Sync: SyncConfig{
			AdzunaInterval:   adzunaInterval,
			RemotiveInterval: remotiveInterval,
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

func requiredString(values map[string]string, key string) (string, error) {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func requiredPositiveInt(values map[string]string, key string) (int, error) {
	value, err := requiredString(values, key)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}

func requiredDuration(values map[string]string, key string) (time.Duration, error) {
	value, err := requiredString(values, key)
	if err != nil {
		return 0, err
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration", key)
	}
	return parsed, nil
}

func requiredWorkplace(values map[string]string) (string, error) {
	workplace, err := requiredString(values, "ADZUNA_WORKPLACE")
	if err != nil {
		return "", err
	}
	switch workplace {
	case "any", "remote", "remote-hybrid":
		return workplace, nil
	default:
		return "", fmt.Errorf("ADZUNA_WORKPLACE must be any, remote, or remote-hybrid")
	}
}
