// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"
	"os"
)

func getDockerSocketPath(configDockerHost string) (string, error) {
	var socketPath string

	// a `-` means don't mount the docker socket to job containers
	if configDockerHost != "" && configDockerHost != "-" {
		socketPath = configDockerHost
	} else {
		socket, found := os.LookupEnv("DOCKER_HOST")
		if !found {
			return "", fmt.Errorf("daemon Docker Engine socket not found and docker_host config was invalid")
		} else {
			socketPath = socket
		}
	}

	return socketPath, nil
}
