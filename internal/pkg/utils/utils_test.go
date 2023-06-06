// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAreStrSlicesElemsEqual(t *testing.T) {
	tests := []struct {
		s1     []string
		s2     []string
		expect bool
	}{
		{
			s1:     []string{"macos-arm64:host", "windows-amd64:host", "ubuntu-latest:docker://node:16-bullseye"},
			s2:     []string{"macos-arm64:host", "windows-amd64:host", "ubuntu-latest:docker://node:16-bullseye"},
			expect: true,
		},
		{
			s1:     []string{"macos-arm64:host", "windows-amd64:host", "ubuntu-latest:docker://node:16-bullseye"},
			s2:     []string{"macos-arm64:host", "windows-amd64:host"},
			expect: false,
		},
		{
			s1:     []string{"macos-arm64:host", "windows-amd64:host", "ubuntu-latest:docker://node:16-bullseye"},
			s2:     []string{"windows-amd64:host", "ubuntu-latest:docker://node:16-bullseye", "macos-arm64:host"},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run("test", func(t *testing.T) {
			actual := AreStrSlicesElemsEqual(tt.s1, tt.s2)
			assert.Equal(t, actual, tt.expect)
		})
	}
}
