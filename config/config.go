// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"gitea.com/gitea/act_runner/core"
)

type (
	// OldConfig provides the system configuration.
	OldConfig struct {
		Debug    bool `envconfig:"GITEA_DEBUG"`
		Trace    bool `envconfig:"GITEA_TRACE"`
		Client   Client
		Runner   Runner
		Platform Platform
	}

	Client struct {
		Address  string `ignored:"true"`
		Insecure bool
	}

	Runner struct {
		UUID     string            `ignored:"true"`
		Name     string            `envconfig:"GITEA_RUNNER_NAME"`
		Token    string            `ignored:"true"`
		Capacity int               `envconfig:"GITEA_RUNNER_CAPACITY" default:"1"`
		File     string            `envconfig:"GITEA_RUNNER_FILE" default:".runner"`
		Environ  map[string]string `envconfig:"GITEA_RUNNER_ENVIRON"`
		EnvFile  string            `envconfig:"GITEA_RUNNER_ENV_FILE"`
		Labels   []string          `envconfig:"GITEA_RUNNER_LABELS"`
	}

	Platform struct {
		OS   string `envconfig:"GITEA_PLATFORM_OS"`
		Arch string `envconfig:"GITEA_PLATFORM_ARCH"`
	}
)

// FromEnviron returns the settings from the environment.
func FromEnviron() (OldConfig, error) {
	cfg := OldConfig{}
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, err
	}

	// check runner config exist
	f, err := os.Stat(cfg.Runner.File)
	if err == nil && !f.IsDir() {
		jsonFile, _ := os.Open(cfg.Runner.File)
		defer jsonFile.Close()
		byteValue, _ := io.ReadAll(jsonFile)
		var runner core.Runner
		if err := json.Unmarshal(byteValue, &runner); err != nil {
			return cfg, err
		}
		if runner.UUID != "" {
			cfg.Runner.UUID = runner.UUID
		}
		if runner.Name != "" {
			cfg.Runner.Name = runner.Name
		}
		if runner.Token != "" {
			cfg.Runner.Token = runner.Token
		}
		if len(runner.Labels) != 0 {
			cfg.Runner.Labels = runner.Labels
		}
		if runner.Address != "" {
			cfg.Client.Address = runner.Address
		}
		if runner.Insecure != "" {
			cfg.Client.Insecure, _ = strconv.ParseBool(runner.Insecure)
		}
	} else if err != nil {
		return cfg, err
	}

	// runner config
	if cfg.Runner.Environ == nil {
		cfg.Runner.Environ = map[string]string{
			"GITHUB_API_URL":    cfg.Client.Address + "/api/v1",
			"GITHUB_SERVER_URL": cfg.Client.Address,
		}
	}
	if cfg.Runner.Name == "" {
		cfg.Runner.Name, _ = os.Hostname()
	}

	// platform config
	if cfg.Platform.OS == "" {
		cfg.Platform.OS = runtime.GOOS
	}
	if cfg.Platform.Arch == "" {
		cfg.Platform.Arch = runtime.GOARCH
	}

	if file := cfg.Runner.EnvFile; file != "" {
		envs, err := godotenv.Read(file)
		if err != nil {
			return cfg, err
		}
		for k, v := range envs {
			cfg.Runner.Environ[k] = v
		}
	}

	return cfg, nil
}

type Config struct {
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	Runner struct {
		File     string            `yaml:"file"`
		Capacity int               `yaml:"capacity"`
		Envs     map[string]string `yaml:"envs"`
		EnvFile  string            `yaml:"env_file"`
		Timeout  time.Duration     `yaml:"timeout"`
	} `yaml:"runner"`
	Cache struct {
		Enabled bool   `yaml:"enabled"`
		Dir     string `yaml:"dir"`
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
	} `yaml:"cache"`
}

// DefaultConfig returns the default configuration.
// If file is not empty, it will be used to load the configuration.
func DefaultConfig(file string) (*Config, error) {
	cfg := &Config{}
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		decoder := yaml.NewDecoder(f)
		if err := decoder.Decode(&cfg); err != nil {
			return nil, err
		}
	}
	compatibleWithOldEnvs(file != "", cfg)
	return cfg, nil
}

// Deprecated: could be removed in the future.
// Be compatible with old envs.
func compatibleWithOldEnvs(fileUsed bool, cfg *Config) {
	handleEnv := func(key string) (string, bool) {
		if v, ok := os.LookupEnv(key); ok {
			if fileUsed {
				log.Warn("env %q has been ignored because config file is used", key)
				return "", false
			}
			log.Warn("env %q will be deprecated, please use config file instead", key)
			return v, true
		}
		return "", false
	}

	if v, ok := handleEnv("GITEA_DEBUG"); ok {
		if b, _ := strconv.ParseBool(v); b {
			cfg.Log.Level = "debug"
		}
	}
	if v, ok := handleEnv("GITEA_TRACE"); ok {
		if b, _ := strconv.ParseBool(v); b {
			cfg.Log.Level = "trace"
		}
	}
	if v, ok := handleEnv("GITEA_RUNNER_CAPACITY"); ok {
		if i, _ := strconv.Atoi(v); i > 0 {
			cfg.Runner.Capacity = i
		}
	}
	if v, ok := handleEnv("GITEA_RUNNER_FILE"); ok {
		cfg.Runner.File = v
	}
	if v, ok := handleEnv("GITEA_RUNNER_ENVIRON"); ok {
		splits := strings.Split(v, ",")
		if cfg.Runner.Envs == nil {
			cfg.Runner.Envs = map[string]string{}
		}
		for _, split := range splits {
			kv := strings.SplitN(split, ":", 2)
			if len(kv) == 2 && kv[0] != "" {
				cfg.Runner.Envs[kv[0]] = kv[1]
			}
		}
	}
	if v, ok := handleEnv("GITEA_RUNNER_ENV_FILE"); ok {
		cfg.Runner.EnvFile = v
	}
}
