// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func getDockerSocketPath(configDockerHost string) (string, error) {
	var socketPath string

	// a `-` means don't mount the docker socket to job containers
	if configDockerHost != "" && configDockerHost != "-" {
		socketPath = configDockerHost
	} else {
		socket, found := socketLocation()
		if !found {
			return "", fmt.Errorf("daemon Docker Engine socket not found and docker_host config was invalid")
		} else {
			socketPath = socket
		}
	}

	return socketPath, nil
}

var commonSocketPaths = []string{
	"/var/run/docker.sock",
	"/var/run/podman/podman.sock",
	"$HOME/.colima/docker.sock",
	"$XDG_RUNTIME_DIR/docker.sock",
	`\\.\pipe\docker_engine`,
	"$HOME/.docker/run/docker.sock",
}

// returns socket path or false if not found any
func socketLocation() (string, bool) {
	if dockerHost, exists := os.LookupEnv("DOCKER_HOST"); exists {
		return dockerHost, true
	}

	for _, p := range commonSocketPaths {
		if _, err := os.Lstat(os.ExpandEnv(p)); err == nil {
			if strings.HasPrefix(p, `\\.\`) {
				return "npipe://" + filepath.ToSlash(os.ExpandEnv(p)), true
			}
			return "unix://" + filepath.ToSlash(os.ExpandEnv(p)), true
		}
	}

	return "", false
}
