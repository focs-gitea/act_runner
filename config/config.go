// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

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
		Insecure bool              `yaml:"insecure"`
	} `yaml:"runner"`
	Cache struct {
		Enabled *bool  `yaml:"enabled"` // pointer to distinguish between false and not set, and it will be true if not set
		Dir     string `yaml:"dir"`
		Host    string `yaml:"host"`
		Port    int    `yaml:"port"`
	} `yaml:"cache"`
}

// LoadDefault returns the default configuration.
// If file is not empty, it will be used to load the configuration.
func LoadDefault(file string) (*Config, error) {
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

	if cfg.Runner.EnvFile != "" {
		if stat, err := os.Stat(cfg.Runner.EnvFile); err == nil && !stat.IsDir() {
			envs, err := godotenv.Read(cfg.Runner.EnvFile)
			if err != nil {
				return nil, fmt.Errorf("read env file %q: %w", cfg.Runner.EnvFile, err)
			}
			for k, v := range envs {
				cfg.Runner.Envs[k] = v
			}
		}
	}

	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Runner.File == "" {
		cfg.Runner.File = ".runner"
	}
	if cfg.Runner.Capacity <= 0 {
		cfg.Runner.Capacity = 1
	}
	if cfg.Runner.Timeout <= 0 {
		cfg.Runner.Timeout = 3 * time.Hour
	}
	if cfg.Cache.Enabled == nil {
		b := true
		cfg.Cache.Enabled = &b
	}
	if *cfg.Cache.Enabled {
		if cfg.Cache.Dir == "" {
			home, _ := os.UserHomeDir()
			cfg.Cache.Dir = filepath.Join(home, ".cache", "actcache")
		}
	}

	return cfg, nil
}

// Deprecated: could be removed in the future.
// Be compatible with old envs.
func compatibleWithOldEnvs(fileUsed bool, cfg *Config) {
	handleEnv := func(key string) (string, bool) {
		if v, ok := os.LookupEnv(key); ok {
			if fileUsed {
				log.Warnf("env %s has been ignored because config file is used", key)
				return "", false
			}
			log.Warnf("env %s will be deprecated, please use config file instead", key)
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
