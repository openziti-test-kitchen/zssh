package config

import (
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type OIDC struct {
	CallbackPort string `yaml:"callback_port"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Issuer       string `yaml:"issuer"`
	Enabled      bool   `yaml:"enabled"`
}

type Config struct {
	SshKeyPath string `yaml:"ssh_key_path"`
	ZConfig    string `yaml:"zconfig"`
	Debug      bool   `yaml:"debug"`
	Service    string `yaml:"service"`
	OIDC       OIDC   `yaml:"oidc"`
	Username   string `yaml:"user"`
}

type ConfigMap map[string]Config

func ConfigHome() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return configHome
}

// GetConfigFilePath returns the path to the config file in the ~/.config directory.
func GetConfigFilePath() string {
	return filepath.Join(ConfigHome(), "zssh", "config.yaml")
}

// DefaultIdentityFile returns the path to the config file in the ~/.config directory.
func DefaultIdentityFile() string {
	return filepath.Join(ConfigHome(), "zssh", "default.json")
}

// LoadConfigs loads the configuration array from a YAML file.
func LoadConfigs(filePath string) (ConfigMap, error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var configs ConfigMap
	if err := yaml.Unmarshal(file, &configs); err != nil {
		return nil, err
	}

	return configs, nil
}

// SaveConfigs saves the configuration array to a YAML file.
func SaveConfigs(configs []Config, filePath string) error {
	data, err := yaml.Marshal(configs)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	return nil
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		SshKeyPath: filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa"),
		ZConfig:    filepath.Join(os.Getenv("HOME"), ".ziti", "zssh.json"),
		Debug:      false,
		Service:    "zssh",
		OIDC: OIDC{
			CallbackPort: "63275",
			ClientID:     "openziti-client",
			ClientSecret: "",
			Issuer:       "https://dev-yourid.okta.com",
			Enabled:      false,
		},
	}
}

// FindConfigByKey finds a configuration by the targetIdentity/key
func FindConfigByKey(key string) *Config {
	configs := LoadConfigFile()
	if cfg, exists := configs[key]; exists {
		return &cfg
	}
	return DefaultConfig()
}

func LoadConfigFile() ConfigMap {
	configFilePath := GetConfigFilePath()
	// Load the configurations from the file, or use defaults if the file doesn't exist
	configs, err := LoadConfigs(configFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Fatalf("Error loading config: %v", err)
		}
	}
	return configs
}
