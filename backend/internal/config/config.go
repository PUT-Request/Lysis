package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	LLM          LLMConfig          `yaml:"llm"`
	LLMSecondary LLMConfig          `yaml:"llm_fallback"`
	Auth         AuthConfig         `yaml:"auth"`
	Sandbox      SandboxConfig      `yaml:"sandbox"`
	Limits       LimitsConfig       `yaml:"limits"`
	Disk         DiskConfig         `yaml:"disk"`
	ExternalAPIs ExternalAPIsConfig `yaml:"external_apis"`
	Cache        CacheConfig        `yaml:"cache"`
	Prompts      PromptsConfig      `yaml:"prompts"`
	Database     DatabaseConfig     `yaml:"database"`
}

type ServerConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	StaticDir string `yaml:"static_dir"`
}

type LLMConfig struct {
	Provider         string  `yaml:"provider"`
	APIKey           string  `yaml:"api_key"`
	Model            string  `yaml:"model"`
	Endpoint         string  `yaml:"endpoint"`
	MaxTokens        int     `yaml:"max_tokens"`
	ContextWindow    int     `yaml:"context_window"`
	MaxContextChars  int     `yaml:"max_context_chars"`
	MaxIterations    int     `yaml:"max_iterations"`
	Temperature      float64 `yaml:"temperature"`
	MaxRetries       int     `yaml:"max_retries"`
}

type AuthConfig struct {
	JWTSecret         string `yaml:"jwt_secret"`
	AccessTTL         string `yaml:"session_ttl"`
	PasswordBcryptCost int    `yaml:"password_bcrypt_cost"`
}

type SandboxConfig struct {
	TempDir             string   `yaml:"temp_dir"`
	CleanupOnComplete   bool     `yaml:"cleanup_on_complete"`
	MaxDiskUsageGB      int      `yaml:"max_disk_usage_gb"`
	BwrapPath           string   `yaml:"bwrap_path"`
	CommandTimeoutSecs  int      `yaml:"command_timeout_seconds"`
	MaxMemoryMB         int      `yaml:"max_memory_mb"`
	Tools               []string `yaml:"tools"`
}

type LimitsConfig struct {
	ScansPerUserPerDay   int    `yaml:"scans_per_day_per_user"`
	ScansPerIPPerDay     int    `yaml:"scans_per_day_per_ip"`
	MaxGlobalConcurrent  int    `yaml:"max_parallel_scans"`
	MaxConcurrentPerUser int    `yaml:"max_parallel_scans_per_user"`
	MaxFileSizeMB        int    `yaml:"max_file_size_mb"`
	MaxZipUncompressedMB int    `yaml:"max_zip_size_mb"`
	ScanTimeout          string `yaml:"scan_timeout"`
}

type DiskConfig struct {
	MinFreeBytes int64 `yaml:"min_free_bytes"`
}

type ExternalAPI struct {
	Enabled     bool   `yaml:"enabled"`
	APIKey      string `yaml:"api_key"`
	ThrottleRPM int    `yaml:"throttle_rpm"`
}

type ExternalAPIsConfig struct {
	VirusTotal ExternalAPI `yaml:"virustotal"`
	AbuseCh    ExternalAPI `yaml:"abusech"`
}

type CacheConfig struct {
	HashTTLHours int `yaml:"hash_ttl_hours"`
}

type PromptConfig struct {
	SystemPrompt string                       `yaml:"system_prompt"`
	ResultSchema map[string]interface{}       `yaml:"result_schema"`
}

type PromptsConfig struct {
	Exploit  PromptConfig `yaml:"exploit"`
	Malware  PromptConfig `yaml:"malware"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.setDefaults()
	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.Sandbox.BwrapPath == "" {
		c.Sandbox.BwrapPath = "/usr/bin/bwrap"
	}
	if c.Sandbox.CommandTimeoutSecs == 0 {
		c.Sandbox.CommandTimeoutSecs = 60
	}
	if c.Sandbox.MaxMemoryMB == 0 {
		c.Sandbox.MaxMemoryMB = 512
	}
	if c.Limits.MaxZipUncompressedMB == 0 {
		c.Limits.MaxZipUncompressedMB = 100
	}
	if c.Limits.MaxFileSizeMB == 0 {
		c.Limits.MaxFileSizeMB = 50
	}
	if c.LLM.MaxRetries == 0 {
		c.LLM.MaxRetries = 3
	}
	if c.Disk.MinFreeBytes == 0 {
		c.Disk.MinFreeBytes = 10 * 1024 * 1024 * 1024 // 10GB
	}
	if c.Auth.PasswordBcryptCost == 0 {
		c.Auth.PasswordBcryptCost = 12
	}
	if c.Limits.MaxGlobalConcurrent == 0 {
		c.Limits.MaxGlobalConcurrent = 100
	}
	if c.Limits.MaxConcurrentPerUser == 0 {
		c.Limits.MaxConcurrentPerUser = 2
	}
}
