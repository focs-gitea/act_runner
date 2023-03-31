package config

import _ "embed"

var (
	//go:embed config.example.yaml
	Example []byte
)
