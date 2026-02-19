package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const invalidValue = "invalid"

func TestLoadConfig_Defaults(t *testing.T) {
	// Empty path: defaults only, no file read
	cfg, err := LoadConfig("")
	require.NoError(t, err)
	assert.Equal(t, DefaultModel, cfg.Model)
	assert.Equal(t, OrlaOutputFormatAuto, cfg.OutputFormat)
}

func TestLoadConfig_WithSpecificPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom.yaml")
	configContent := "log_format: pretty\nlog_level: debug\n"
	// #nosec G306 -- test file permissions are acceptable for temporary test files
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, OrlaLogFormatPretty, cfg.LogFormat)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoadConfig_InvalidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	// #nosec G306 -- test file permissions are acceptable for temporary test files
	require.NoError(t, os.WriteFile(configPath, []byte("invalid: yaml: content: [unclosed"), 0644))

	_, err := LoadConfig(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestValidateConfig(t *testing.T) {
	cfg := &OrlaConfig{
		LogFormat:    OrlaLogFormatJSON,
		LogLevel:     "info",
		Model:        DefaultModel,
		OutputFormat: OrlaOutputFormatAuto,
	}

	err := validateConfig(cfg)
	require.NoError(t, err)

	// Test invalid log format
	cfg.LogFormat = invalidValue
	err = validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_format must be one of")

	// Test invalid log level
	cfg.LogFormat = "json"
	cfg.LogLevel = invalidValue
	err = validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log_level must be one of")

	// Test invalid output_format
	cfg.LogLevel = "info"
	cfg.OutputFormat = invalidValue
	err = validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output_format must be one of")
}

func TestLoadConfig_WithYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yaml")
	configContent := `
log_format: pretty
log_level: debug
model: openai:gpt-4
streaming: false
output_format: rich
`
	// #nosec G306 -- test file permissions are acceptable for temporary test files
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, OrlaLogFormatPretty, cfg.LogFormat)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "openai:gpt-4", cfg.Model)
	assert.Equal(t, false, cfg.Streaming)
	assert.Equal(t, OrlaOutputFormatRich, cfg.OutputFormat)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	// #nosec G306 -- test file permissions are acceptable for temporary test files
	require.NoError(t, os.WriteFile(configPath, []byte("invalid: yaml: [unclosed"), 0644))

	_, err := LoadConfig(configPath)
	require.Error(t, err)
}

func TestValidateConfig_Defaults(t *testing.T) {
	// Test that validateConfig errors on empty/zero values when called directly
	cfg := &OrlaConfig{}
	err := validateConfig(cfg)
	require.Error(t, err)

	// Load a config file with only log_format set; other fields get defaults
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "orla.yaml")
	configContent := "log_format: json\n"
	// #nosec G306 -- test file permissions are acceptable for temporary test files
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg2, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, DefaultModel, cfg2.Model)
	assert.Equal(t, OrlaOutputFormatAuto, cfg2.OutputFormat)
}

func TestValidateConfig_BadValues(t *testing.T) {
	cfg := &OrlaConfig{
		LogFormat:    OrlaLogFormatJSON,
		LogLevel:     "info",
		Model:        "test",
		OutputFormat: OrlaOutputFormatAuto,
	}

	cfg.OutputFormat = "invalid"
	err := validateConfig(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output_format must be one of")
}

func TestConfig_MarshalUnmarshal(t *testing.T) {
	cfg := &OrlaConfig{
		LogFormat: OrlaLogFormatJSON,
		LogLevel:  "info",
		Model:     "openai:gpt-4",
	}

	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var cfg2 OrlaConfig
	err = yaml.Unmarshal(data, &cfg2)
	require.NoError(t, err)

	assert.Equal(t, cfg.LogFormat, cfg2.LogFormat)
	assert.Equal(t, cfg.Model, cfg2.Model)
}
