// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runtime

import (
	"context"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"

	"gitea.com/gitea/act_runner/artifactcache"
	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/labels"
)

// Runner runs the pipeline.
type Runner struct {
	Machine       string
	Version       string
	ForgeInstance string
	Environ       map[string]string
	Client        client.Client
	Labels        labels.Labels
	Network       string
	CacheHandler  *artifactcache.Handler
}

// Run runs the pipeline stage.
func (s *Runner) Run(ctx context.Context, task *runnerv1.Task) error {
	env := map[string]string{}
	for k, v := range s.Environ {
		env[k] = v
	}
	if s.CacheHandler != nil {
		env["ACTIONS_CACHE_URL"] = s.CacheHandler.ExternalURL() + "/"
	}
	return NewTask(task.Id, s.Client, env, s.Network, s.Labels.PickPlatform).Run(ctx, task, s.Machine, s.Version)
}
