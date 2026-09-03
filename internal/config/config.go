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
	OpenCode     OpenCodeConfig
	OpenAI       OpenAIConfig
	JobMatch     JobMatchConfig
	Events       EventConfig
	OpenRouter   OpenRouterConfig
}

type ProviderConfig struct {
	Adzuna   AdzunaConfig
	Jobicy   JobicyConfig
	LinkedIn LinkedInConfig
	Remotive RemotiveConfig
}

type OpenCodeConfig struct {
	BaseURL        string
	APIKey         string
	JobAnalysis    TaskConfig
	ProfileMatcher TaskConfig
}

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	JobChat TaskConfig
}

type TaskConfig struct {
	Model           string
	ReasoningEffort string
}

type JobMatchConfig struct {
	RunInterval time.Duration
}

type EventConfig struct {
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
}

type OpenRouterConfig struct {
	APIKey string
}

type AdzunaConfig struct {
	AppID        string
	APIKey       string
	SyncInterval time.Duration
}

type RemotiveConfig struct {
	SyncInterval time.Duration
}

type JobicyConfig struct {
	SyncInterval time.Duration
}

type LinkedInConfig struct {
	SyncInterval    time.Duration
	RequestInterval time.Duration
}

type fileConfig struct {
	Server struct {
		HTTPAddress string `toml:"http_address" validate:"notblank"`
	} `toml:"server"`
	Database struct {
		Path string `toml:"path" validate:"notblank"`
	} `toml:"database"`
	OpenCode  fileOpenCodeConfig `toml:"opencode"`
	OpenAI    fileOpenAIConfig   `toml:"openai"`
	JobMatch  fileJobMatchConfig `toml:"job_match"`
	Events    fileEventConfig    `toml:"events"`
	Providers struct {
		Adzuna   fileAdzunaConfig   `toml:"adzuna"`
		Jobicy   fileJobicyConfig   `toml:"jobicy"`
		LinkedIn fileLinkedInConfig `toml:"linkedin"`
		Remotive fileRemotiveConfig `toml:"remotive"`
	} `toml:"providers"`
}

type fileOpenCodeConfig struct {
	BaseURL        string         `toml:"base_url" validate:"notblank,url"`
	JobAnalysis    fileTaskConfig `toml:"job_analysis"`
	ProfileMatcher fileTaskConfig `toml:"profile_matcher"`
}

type fileOpenAIConfig struct {
	BaseURL string         `toml:"base_url" validate:"notblank,url"`
	JobChat fileTaskConfig `toml:"job_chat"`
}

type fileTaskConfig struct {
	Model           string `toml:"model" validate:"notblank"`
	ReasoningEffort string `toml:"reasoning_effort" validate:"oneof=low medium high"`
}

type fileJobMatchConfig struct {
	RunInterval string `toml:"run_interval" validate:"notblank,duration"`
}

type fileEventConfig struct {
	QueueSize     int    `toml:"queue_size" validate:"gte=1"`
	BatchSize     int    `toml:"batch_size" validate:"gte=1"`
	FlushInterval string `toml:"flush_interval" validate:"notblank,duration"`
}

type fileAdzunaConfig struct {
	SyncInterval string `toml:"sync_interval" validate:"notblank,duration"`
}

type fileRemotiveConfig struct {
	SyncInterval string `toml:"sync_interval" validate:"notblank,duration"`
}

type fileJobicyConfig struct {
	SyncInterval string `toml:"sync_interval" validate:"notblank,duration"`
}

type fileLinkedInConfig struct {
	SyncInterval    string `toml:"sync_interval" validate:"notblank,duration"`
	RequestInterval string `toml:"request_interval" validate:"notblank,duration"`
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
	codexProxyAPIKey, err := requiredString(tokens["CODEX_PROXY_KEY"], "CODEX_PROXY_KEY")
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
	linkedInInterval, err := time.ParseDuration(source.Providers.LinkedIn.SyncInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse providers.linkedin.sync_interval: %w", err)
	}
	linkedInRequestInterval, err := time.ParseDuration(source.Providers.LinkedIn.RequestInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse providers.linkedin.request_interval: %w", err)
	}
	jobMatchRunInterval, err := time.ParseDuration(source.JobMatch.RunInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse job_match.run_interval: %w", err)
	}
	eventFlushInterval, err := time.ParseDuration(source.Events.FlushInterval)
	if err != nil {
		return Config{}, fmt.Errorf("parse events.flush_interval: %w", err)
	}
	if jobicyInterval < time.Hour {
		return Config{}, fmt.Errorf("providers.jobicy.sync_interval must be at least 1h")
	}
	if linkedInInterval < time.Hour {
		return Config{}, fmt.Errorf("providers.linkedin.sync_interval must be at least 1h")
	}

	return Config{
		HTTPAddress:  source.Server.HTTPAddress,
		DatabasePath: source.Database.Path,
		Providers: ProviderConfig{
			Adzuna: AdzunaConfig{
				AppID:        appID,
				APIKey:       apiKey,
				SyncInterval: adzunaInterval,
			},
			Remotive: RemotiveConfig{
				SyncInterval: remotiveInterval,
			},
			Jobicy: JobicyConfig{
				SyncInterval: jobicyInterval,
			},
			LinkedIn: LinkedInConfig{
				SyncInterval:    linkedInInterval,
				RequestInterval: linkedInRequestInterval,
			},
		},
		OpenCode: OpenCodeConfig{
			BaseURL: source.OpenCode.BaseURL,
			APIKey:  llmAPIKey,
			JobAnalysis: TaskConfig{
				Model:           source.OpenCode.JobAnalysis.Model,
				ReasoningEffort: source.OpenCode.JobAnalysis.ReasoningEffort,
			},
			ProfileMatcher: TaskConfig{
				Model:           source.OpenCode.ProfileMatcher.Model,
				ReasoningEffort: source.OpenCode.ProfileMatcher.ReasoningEffort,
			},
		},
		OpenAI: OpenAIConfig{
			BaseURL: source.OpenAI.BaseURL,
			APIKey:  codexProxyAPIKey,
			JobChat: TaskConfig{
				Model:           source.OpenAI.JobChat.Model,
				ReasoningEffort: source.OpenAI.JobChat.ReasoningEffort,
			},
		},
		JobMatch:   JobMatchConfig{RunInterval: jobMatchRunInterval},
		Events:     EventConfig{QueueSize: source.Events.QueueSize, BatchSize: source.Events.BatchSize, FlushInterval: eventFlushInterval},
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
