// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package poll

import (
	"context"
	"errors"
	"sync"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"gitea.com/gitea/act_runner/internal/app/run"
	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
)

type Poller struct {
	client client.Client
	runner *run.Runner
	cfg    *config.Config
}

// taskVersion used to store the version of the last task fetched from the Gitea.
var taskVersion int64

func New(cfg *config.Config, client client.Client, runner *run.Runner) *Poller {
	return &Poller{
		client: client,
		runner: runner,
		cfg:    cfg,
	}
}

func (p *Poller) Poll(ctx context.Context) {
	limiter := rate.NewLimiter(rate.Every(p.cfg.Runner.FetchInterval), 1)
	wg := &sync.WaitGroup{}
	for i := 0; i < p.cfg.Runner.Capacity; i++ {
		wg.Add(1)
		go p.poll(ctx, wg, limiter)
	}
	wg.Wait()
}

func (p *Poller) poll(ctx context.Context, wg *sync.WaitGroup, limiter *rate.Limiter) {
	defer wg.Done()
	for {
		if err := limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				log.WithError(err).Debug("limiter wait failed")
			}
			return
		}
		task, ok := p.fetchTask(ctx)
		if !ok {
			continue
		}
		if err := p.runner.Run(ctx, task); err != nil {
			log.WithError(err).Error("failed to run task")
		}
	}
}

func (p *Poller) fetchTask(ctx context.Context) (*runnerv1.Task, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, p.cfg.Runner.FetchTimeout)
	defer cancel()

	resp, err := p.client.FetchTask(reqCtx, connect.NewRequest(&runnerv1.FetchTaskRequest{
		TaskVersion: taskVersion,
	}))
	if errors.Is(err, context.DeadlineExceeded) {
		err = nil
	}
	if err != nil {
		log.WithError(err).Error("failed to fetch task")
		return nil, false
	}

	if resp == nil || resp.Msg == nil {
		return nil, false
	}

	taskVersion = resp.Msg.TaskVersion

	if resp.Msg.Task == nil {
		return nil, false
	}

	// got a task, set `taskVersion` to zero to focre query db in next request.
	taskVersion = 0

	return resp.Msg.Task, true
}
