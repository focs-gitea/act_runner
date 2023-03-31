// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package config

import _ "embed"

var (
	//go:embed config.example.yaml
	Example []byte
)
