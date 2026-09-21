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
	tokensEnvFile = ".env.tokens" // #nosec G101 -- this is the configured local credentials file name.
)

type Config struct {
	HTTPAddress       string
	MetricsSocketPath string
	DatabasePath      string
	Providers         ProviderConfig
	OpenCode          OpenCodeConfig
	OpenAI            OpenAIConfig
	TypeSafe          TypeSafeConfig
	JobAnalysis       TaskConfig
	CustomJobImport   TaskConfig
	ProfileMatcher    TaskConfig
	JobChat           TaskConfig
	JobMatch          JobMatchConfig
	Events            EventConfig
	OpenRouter        OpenRouterConfig
}

type ProviderConfig struct {
	Adzuna   AdzunaConfig
	Jobicy   JobicyConfig
	LinkedIn LinkedInConfig
	Remotive RemotiveConfig
}

type OpenCodeConfig struct {
	BaseURL string
	APIKey  string
}

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
}

type TypeSafeConfig struct {
	BaseURL string
	Model   string
	APIKey  string
}

type TaskConfig struct {
	Provider        string
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
	SyncInterval           time.Duration
	RequestInterval        time.Duration
	PreviewRequestInterval time.Duration
}

type configCredentials struct {
	adzunaAppID      string
	adzunaAPIKey     string
	openRouterAPIKey string
	openCodeAPIKey   string
	codexProxyAPIKey string
	typeSafeAPIKey   string
}

type configDurations struct {
	adzunaInterval                 time.Duration
	remotiveInterval               time.Duration
	jobicyInterval                 time.Duration
	linkedInInterval               time.Duration
	linkedInRequestInterval        time.Duration
	linkedInPreviewRequestInterval time.Duration
	jobMatchRunInterval            time.Duration
	eventFlushInterval             time.Duration
}

type fileConfig struct {
	Server struct {
		HTTPAddress       string `toml:"http_address" validate:"notblank"`
		MetricsSocketPath string `toml:"metrics_socket_path" validate:"notblank"`
	} `toml:"server"`
	Database struct {
		Path string `toml:"path" validate:"notblank"`
	} `toml:"database"`
	OpenCode        fileOpenCodeConfig `toml:"opencode"`
	OpenAI          fileOpenAIConfig   `toml:"openai"`
	TypeSafe        fileTypeSafeConfig `toml:"typesafe"`
	JobAnalysis     fileTaskConfig     `toml:"job_analysis"`
	CustomJobImport fileTaskConfig     `toml:"custom_job_import"`
	ProfileMatcher  fileTaskConfig     `toml:"profile_matcher"`
	JobChat         fileTaskConfig     `toml:"job_chat"`
	JobMatch        fileJobMatchConfig `toml:"job_match"`
	Events          fileEventConfig    `toml:"events"`
	Providers       struct {
		Adzuna   fileAdzunaConfig   `toml:"adzuna"`
		Jobicy   fileJobicyConfig   `toml:"jobicy"`
		LinkedIn fileLinkedInConfig `toml:"linkedin"`
		Remotive fileRemotiveConfig `toml:"remotive"`
	} `toml:"providers"`
}

type fileOpenCodeConfig struct {
	BaseURL string `toml:"base_url" validate:"notblank,url"`
}

type fileOpenAIConfig struct {
	BaseURL string `toml:"base_url" validate:"notblank,url"`
}

type fileTypeSafeConfig struct {
	BaseURL string `toml:"base_url" validate:"notblank,url"`
	Model   string `toml:"model" validate:"notblank"`
}

type fileTaskConfig struct {
	Provider        string `toml:"provider" validate:"oneof=opencode openai"`
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
	SyncInterval           string `toml:"sync_interval" validate:"notblank,duration"`
	RequestInterval        string `toml:"request_interval" validate:"notblank,duration"`
	PreviewRequestInterval string `toml:"preview_request_interval" validate:"notblank,duration"`
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

	credentials, err := readConfigCredentials(tokens)
	if err != nil {
		return Config{}, err
	}

	durations, err := parseConfigDurations(source)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddress:       source.Server.HTTPAddress,
		MetricsSocketPath: source.Server.MetricsSocketPath,
		DatabasePath:      source.Database.Path,
		Providers: ProviderConfig{
			Adzuna: AdzunaConfig{
				AppID:        credentials.adzunaAppID,
				APIKey:       credentials.adzunaAPIKey,
				SyncInterval: durations.adzunaInterval,
			},
			Remotive: RemotiveConfig{
				SyncInterval: durations.remotiveInterval,
			},
			Jobicy: JobicyConfig{
				SyncInterval: durations.jobicyInterval,
			},
			LinkedIn: LinkedInConfig{
				SyncInterval:           durations.linkedInInterval,
				RequestInterval:        durations.linkedInRequestInterval,
				PreviewRequestInterval: durations.linkedInPreviewRequestInterval,
			},
		},
		OpenCode: OpenCodeConfig{
			BaseURL: source.OpenCode.BaseURL,
			APIKey:  credentials.openCodeAPIKey,
		},
		OpenAI: OpenAIConfig{
			BaseURL: source.OpenAI.BaseURL,
			APIKey:  credentials.codexProxyAPIKey,
		},
		TypeSafe:        TypeSafeConfig{BaseURL: source.TypeSafe.BaseURL, Model: source.TypeSafe.Model, APIKey: credentials.typeSafeAPIKey},
		JobAnalysis:     TaskConfig{Provider: source.JobAnalysis.Provider, Model: source.JobAnalysis.Model, ReasoningEffort: source.JobAnalysis.ReasoningEffort},
		CustomJobImport: TaskConfig{Provider: source.CustomJobImport.Provider, Model: source.CustomJobImport.Model, ReasoningEffort: source.CustomJobImport.ReasoningEffort},
		ProfileMatcher:  TaskConfig{Provider: source.ProfileMatcher.Provider, Model: source.ProfileMatcher.Model, ReasoningEffort: source.ProfileMatcher.ReasoningEffort},
		JobChat:         TaskConfig{Provider: source.JobChat.Provider, Model: source.JobChat.Model, ReasoningEffort: source.JobChat.ReasoningEffort},
		JobMatch:        JobMatchConfig{RunInterval: durations.jobMatchRunInterval},
		Events:          EventConfig{QueueSize: source.Events.QueueSize, BatchSize: source.Events.BatchSize, FlushInterval: durations.eventFlushInterval},
		OpenRouter:      OpenRouterConfig{APIKey: credentials.openRouterAPIKey},
	}, nil
}

func readConfigCredentials(tokens map[string]string) (configCredentials, error) {
	appID, err := requiredString(tokens["ADZUNA_APP_ID"], "ADZUNA_APP_ID")
	if err != nil {
		return configCredentials{}, err
	}

	apiKey, err := requiredString(tokens["ADZUNA_API_KEY"], "ADZUNA_API_KEY")
	if err != nil {
		return configCredentials{}, err
	}

	openRouterAPIKey, err := requiredString(tokens["OPENROUTER_API_KEY"], "OPENROUTER_API_KEY")
	if err != nil {
		return configCredentials{}, err
	}

	openCodeAPIKey, err := requiredString(tokens["OPENCODE_GO_KEY_4"], "OPENCODE_GO_KEY_4")
	if err != nil {
		return configCredentials{}, err
	}

	codexProxyAPIKey, err := requiredString(tokens["CODEX_PROXY_KEY"], "CODEX_PROXY_KEY")
	if err != nil {
		return configCredentials{}, err
	}

	typeSafeAPIKey, err := requiredString(tokens["TYPESAFEAI_API_KEY"], "TYPESAFEAI_API_KEY")
	if err != nil {
		return configCredentials{}, err
	}

	return configCredentials{
		adzunaAppID:      appID,
		adzunaAPIKey:     apiKey,
		openRouterAPIKey: openRouterAPIKey,
		openCodeAPIKey:   openCodeAPIKey,
		codexProxyAPIKey: codexProxyAPIKey,
		typeSafeAPIKey:   typeSafeAPIKey,
	}, nil
}

func parseConfigDurations(source fileConfig) (configDurations, error) {
	adzunaInterval, err := parseDuration(source.Providers.Adzuna.SyncInterval, "providers.adzuna.sync_interval")
	if err != nil {
		return configDurations{}, err
	}

	remotiveInterval, err := parseDuration(source.Providers.Remotive.SyncInterval, "providers.remotive.sync_interval")
	if err != nil {
		return configDurations{}, err
	}

	jobicyInterval, err := parseDuration(source.Providers.Jobicy.SyncInterval, "providers.jobicy.sync_interval")
	if err != nil {
		return configDurations{}, err
	}

	if jobicyInterval < time.Hour {
		return configDurations{}, fmt.Errorf("providers.jobicy.sync_interval must be at least 1h")
	}

	linkedInInterval, err := parseDuration(source.Providers.LinkedIn.SyncInterval, "providers.linkedin.sync_interval")
	if err != nil {
		return configDurations{}, err
	}

	if linkedInInterval < time.Hour {
		return configDurations{}, fmt.Errorf("providers.linkedin.sync_interval must be at least 1h")
	}

	linkedInRequestInterval, err := parseDuration(source.Providers.LinkedIn.RequestInterval, "providers.linkedin.request_interval")
	if err != nil {
		return configDurations{}, err
	}

	linkedInPreviewRequestInterval, err := parseDuration(source.Providers.LinkedIn.PreviewRequestInterval, "providers.linkedin.preview_request_interval")
	if err != nil {
		return configDurations{}, err
	}

	jobMatchRunInterval, err := parseDuration(source.JobMatch.RunInterval, "job_match.run_interval")
	if err != nil {
		return configDurations{}, err
	}

	eventFlushInterval, err := parseDuration(source.Events.FlushInterval, "events.flush_interval")
	if err != nil {
		return configDurations{}, err
	}

	return configDurations{
		adzunaInterval:                 adzunaInterval,
		remotiveInterval:               remotiveInterval,
		jobicyInterval:                 jobicyInterval,
		linkedInInterval:               linkedInInterval,
		linkedInRequestInterval:        linkedInRequestInterval,
		linkedInPreviewRequestInterval: linkedInPreviewRequestInterval,
		jobMatchRunInterval:            jobMatchRunInterval,
		eventFlushInterval:             eventFlushInterval,
	}, nil
}

func parseDuration(value, field string) (time.Duration, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}

	return duration, nil
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
