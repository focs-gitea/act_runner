// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
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
		File          string            `yaml:"file"`
		Capacity      int               `yaml:"capacity"`
		Envs          map[string]string `yaml:"envs"`
		EnvFile       string            `yaml:"env_file"`
		Timeout       time.Duration     `yaml:"timeout"`
		Insecure      bool              `yaml:"insecure"`
		FetchTimeout  time.Duration     `yaml:"fetch_timeout"`
		FetchInterval time.Duration     `yaml:"fetch_interval"`
	} `yaml:"runner"`
	Cache struct {
		Enabled *bool  `yaml:"enabled"` // pointer to distinguish between false and not set, and it will be true if not set
		Dir     string `yaml:"dir"`
		Host    string `yaml:"host"`
		Port    uint16 `yaml:"port"`
	} `yaml:"cache"`
	Container struct {
		Network       string `yaml:"network"`
		NetworkMode   string `yaml:"network_mode"` // Deprecated: use Network instead. Could be removed after Gitea 1.20
		Privileged    bool   `yaml:"privileged"`
		Options       string `yaml:"options"`
		WorkdirParent string `yaml:"workdir_parent"`
	} `yaml:"container"`
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
	if cfg.Container.WorkdirParent == "" {
		cfg.Container.WorkdirParent = "workspace"
	}
	if cfg.Runner.FetchTimeout <= 0 {
		cfg.Runner.FetchTimeout = 5 * time.Second
	}
	if cfg.Runner.FetchInterval <= 0 {
		cfg.Runner.FetchInterval = 2 * time.Second
	}

	// although `container.network_mode` will be deprecated,
	// but we have to be compatible with it for now.
	// Compatible logic:
	// 1. The following logic (point 2 and 3) will only be executed if the value of `container.network_mode` is not empty string and the value of `container.network` is empty string.
	//    If you want to specify `container.network` to empty string to make `act_runner` create a new network for each job,
	//    please make sure that `container.network_mode` is not exist in you configuration file.
	// 2. If the value of `container.network_mode` is `bridge`, then the value of `container.network` will be set to empty string.
	//		`act_runner` will create a new network for each job
	// 3. Otherwise, the value of `container.network` will be set to the value of `container.network_mode`.
	if cfg.Container.NetworkMode != "" && cfg.Container.Network == "" {
		log.Warnf("You are trying to use deprecated configuration item of `container.network_mode`: %s, it will be removed after Gitea 1.20 released", cfg.Container.NetworkMode)
		log.Warn("More information is available in PR (https://gitea.com/gitea/act_runner/pulls/184)")
		if cfg.Container.NetworkMode == "bridge" {
			cfg.Container.Network = ""
		} else {
			cfg.Container.Network = cfg.Container.NetworkMode
		}
		log.Warnf("the value of `container.network` will be set to '%s'", cfg.Container.Network)
	}

	return cfg, nil
}
