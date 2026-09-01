package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/pelletier/go-toml/v2"
)

const (
	configFile    = "config.toml"
	tokensEnvFile = ".env.tokens"
)

type Config struct {
	HTTPAddress  string
	DatabasePath string
	Providers    ProviderConfig
	LLM          LLMConfig
	JobMatch     JobMatchConfig
	OpenRouter   OpenRouterConfig
}

type ProviderConfig struct {
	Adzuna   AdzunaConfig
	Jobicy   JobicyConfig
	Remotive RemotiveConfig
}

type LLMConfig struct {
	BaseURL        string
	APIKey         string
	JobAnalysis    LLMTaskConfig
	ProfileMatcher LLMTaskConfig
}

type LLMTaskConfig struct {
	Model           string
	ReasoningEffort string
}

type JobMatchConfig struct {
	RunInterval time.Duration
}

type OpenRouterConfig struct {
	APIKey string
}

type AdzunaConfig struct {
	AppID          string
	APIKey         string
	Query          string
	Country        string
	MaxDaysOld     int
	MaxPages       int
	ResultsPerPage int
	Workplace      string
	SyncInterval   time.Duration
}

type RemotiveConfig struct {
	Query        string
	SyncInterval time.Duration
}

type JobicyConfig struct {
	Count        int
	Geo          string
	Industry     string
	Tag          string
	SyncInterval time.Duration
}

type fileConfig struct {
	Server struct {
		HTTPAddress string `toml:"http_address" validate:"notblank"`
	} `toml:"server"`
	Database struct {
		Path string `toml:"path" validate:"notblank"`
	} `toml:"database"`
	LLM       fileLLMConfig      `toml:"llm"`
	JobMatch  fileJobMatchConfig `toml:"job_match"`
	Providers struct {
		Adzuna   fileAdzunaConfig   `toml:"adzuna"`
		Jobicy   fileJobicyConfig   `toml:"jobicy"`
		Remotive fileRemotiveConfig `toml:"remotive"`
	} `toml:"providers"`
}

type fileLLMConfig struct {
	BaseURL        string            `toml:"base_url" validate:"notblank,url"`
	JobAnalysis    fileLLMTaskConfig `toml:"job_analysis"`
	ProfileMatcher fileLLMTaskConfig `toml:"profile_matcher"`
}

type fileLLMTaskConfig struct {
	Model           string `toml:"model" validate:"notblank"`
	ReasoningEffort string `toml:"reasoning_effort" validate:"oneof=low medium high"`
}

type fileJobMatchConfig struct {
	RunInterval string `toml:"run_interval" validate:"notblank,duration"`
}

type fileAdzunaConfig struct {
	Query          string `toml:"query" validate:"notblank"`
	Country        string `toml:"country" validate:"notblank"`
	MaxDaysOld     int    `toml:"max_days_old" validate:"gte=1"`
	MaxPages       int    `toml:"max_pages" validate:"gte=1"`
	ResultsPerPage int    `toml:"results_per_page" validate:"gte=1"`
	Workplace      string `toml:"workplace" validate:"oneof=any remote remote-hybrid"`
	SyncInterval   string `toml:"sync_interval" validate:"notblank,duration"`
}

type fileRemotiveConfig struct {
	Query        string `toml:"query" validate:"notblank"`
	SyncInterval string `toml:"sync_interval" validate:"notblank,duration"`
}

type fileJobicyConfig struct {
	Count        int    `toml:"count" validate:"gte=1,lte=200"`
	Geo          string `toml:"geo"`
	Industry     string `toml:"industry"`
	Tag          string `toml:"tag"`
	SyncInterval string `toml:"sync_interval" validate:"notblank,duration"`
}

func Load() (Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var source fileConfig
	decoder := toml.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&source); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", configFile, err)
	}
	if err := validateFileConfig(source); err != nil {
		return Config{}, err
	}

	tokens, err := readEnvFile(tokensEnvFile)
	if err != nil {
		return Config{}, err
	}
	appID, err := requiredString(tokens["ADZUNA_APP_ID"], "ADZUNA_APP_ID")
	if err != nil {
		return Config{}, err
	}
	apiKey, err := requiredString(tokens["ADZUNA_API_KEY"], "ADZUNA_API_KEY")
	if err != nil {
		return Config{}, err
	}
	openRouterAPIKey, err := requiredString(tokens["OPENROUTER_API_KEY"], "OPENROUTER_API_KEY")
	if err != nil {
		return Config{}, err
	}
	llmAPIKey, err := requiredString(tokens["OPENCODE_GO_KEY_4"], "OPENCODE_GO_KEY_4")
	if err != nil {
		return Config{}, err
	}
	adzunaInterval, err := time.ParseDuration(source.Providers.Adzuna.SyncInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse providers.adzuna.sync_interval: %w", err)
	}
	remotiveInterval, err := time.ParseDuration(source.Providers.Remotive.SyncInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse providers.remotive.sync_interval: %w", err)
	}
	jobicyInterval, err := time.ParseDuration(source.Providers.Jobicy.SyncInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse providers.jobicy.sync_interval: %w", err)
	}
	jobMatchRunInterval, err := time.ParseDuration(source.JobMatch.RunInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse job_match.run_interval: %w", err)
	}
	if jobicyInterval < time.Hour {
		return Config{}, fmt.Errorf("providers.jobicy.sync_interval must be at least 1h")
	}

	return Config{
		HTTPAddress:  source.Server.HTTPAddress,
		DatabasePath: source.Database.Path,
		Providers: ProviderConfig{
			Adzuna: AdzunaConfig{
				AppID:          appID,
				APIKey:         apiKey,
				Query:          source.Providers.Adzuna.Query,
				Country:        source.Providers.Adzuna.Country,
				MaxDaysOld:     source.Providers.Adzuna.MaxDaysOld,
				MaxPages:       source.Providers.Adzuna.MaxPages,
				ResultsPerPage: source.Providers.Adzuna.ResultsPerPage,
				Workplace:      source.Providers.Adzuna.Workplace,
				SyncInterval:   adzunaInterval,
			},
			Remotive: RemotiveConfig{
				Query:        source.Providers.Remotive.Query,
				SyncInterval: remotiveInterval,
			},
			Jobicy: JobicyConfig{
				Count:        source.Providers.Jobicy.Count,
				Geo:          source.Providers.Jobicy.Geo,
				Industry:     source.Providers.Jobicy.Industry,
				Tag:          source.Providers.Jobicy.Tag,
				SyncInterval: jobicyInterval,
			},
		},
		LLM: LLMConfig{
			BaseURL: source.LLM.BaseURL,
			APIKey:  llmAPIKey,
			JobAnalysis: LLMTaskConfig{
				Model:           source.LLM.JobAnalysis.Model,
				ReasoningEffort: source.LLM.JobAnalysis.ReasoningEffort,
			},
			ProfileMatcher: LLMTaskConfig{
				Model:           source.LLM.ProfileMatcher.Model,
				ReasoningEffort: source.LLM.ProfileMatcher.ReasoningEffort,
			},
		},
		JobMatch:   JobMatchConfig{RunInterval: jobMatchRunInterval},
		OpenRouter: OpenRouterConfig{APIKey: openRouterAPIKey},
	}, nil
}

func validateFileConfig(source fileConfig) error {
	validate := validator.New()
	if err := validate.RegisterValidation("notblank", isNotBlank); err != nil {
		return fmt.Errorf("register notblank validator: %w", err)
	}
	if err := validate.RegisterValidation("duration", isPositiveDuration); err != nil {
		return fmt.Errorf("register duration validator: %w", err)
	}
	if err := validate.Struct(source); err != nil {
		return fmt.Errorf("validate %s: %w", configFile, err)
	}
	return nil
}

func isNotBlank(level validator.FieldLevel) bool {
	return strings.TrimSpace(level.Field().String()) != ""
}

func isPositiveDuration(level validator.FieldLevel) bool {
	duration, err := time.ParseDuration(level.Field().String())
	return err == nil && duration > 0
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

func requiredString(value, key string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}
