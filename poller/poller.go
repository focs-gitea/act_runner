// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package poller

import (
	"context"
	"errors"
	"sync"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"

	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
)

var ErrDataLock = errors.New("Data Lock Error")

func New(cli client.Client, dispatch func(context.Context, *runnerv1.Task) error, cfg *config.Config) *Poller {
	return &Poller{
		Client:       cli,
		Dispatch:     dispatch,
		routineGroup: newRoutineGroup(),
		metric:       &metric{},
		ready:        make(chan struct{}, 1),
		cfg:          cfg,
	}
}

type Poller struct {
	Client   client.Client
	Dispatch func(context.Context, *runnerv1.Task) error

	sync.Mutex
	routineGroup *routineGroup
	metric       *metric
	ready        chan struct{}
	cfg          *config.Config
}

func (p *Poller) schedule() {
	p.Lock()
	defer p.Unlock()
	if int(p.metric.BusyWorkers()) >= p.cfg.Runner.Capacity {
		return
	}

	select {
	case p.ready <- struct{}{}:
	default:
	}
}

func (p *Poller) Wait() {
	p.routineGroup.Wait()
}

func (p *Poller) handle(ctx context.Context, l *log.Entry) {
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("handle task panic: %+v", r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			task, err := p.pollTask(ctx)
			if task == nil || err != nil {
				if err != nil {
					l.Errorf("can't find the task: %v", err.Error())
				}
				time.Sleep(5 * time.Second)
				break
			}

			p.metric.IncBusyWorker()
			p.routineGroup.Run(func() {
				defer p.schedule()
				defer p.metric.DecBusyWorker()
				if err := p.dispatchTask(ctx, task); err != nil {
					l.Errorf("execute task: %v", err.Error())
				}
			})
			return
		}
	}
}

func (p *Poller) Poll(ctx context.Context) error {
	l := log.WithField("func", "Poll")

	for {
		// check worker number
		p.schedule()

		select {
		// wait worker ready
		case <-p.ready:
		case <-ctx.Done():
			return nil
		}
		p.handle(ctx, l)
	}
}

func (p *Poller) pollTask(ctx context.Context) (*runnerv1.Task, error) {
	l := log.WithField("func", "pollTask")
	l.Info("poller: request stage from remote server")

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// request a new build stage for execution from the central
	// build server.
	resp, err := p.Client.FetchTask(reqCtx, connect.NewRequest(&runnerv1.FetchTaskRequest{}))
	if err == context.Canceled || err == context.DeadlineExceeded {
		l.WithError(err).Trace("poller: no stage returned")
		return nil, nil
	}

	if err != nil && err == ErrDataLock {
		l.WithError(err).Info("task accepted by another runner")
		return nil, nil
	}

	if err != nil {
		l.WithError(err).Error("cannot accept task")
		return nil, err
	}

	// exit if a nil or empty stage is returned from the system
	// and allow the runner to retry.
	if resp.Msg.Task == nil || resp.Msg.Task.Id == 0 {
		return nil, nil
	}

	return resp.Msg.Task, nil
}

func (p *Poller) dispatchTask(ctx context.Context, task *runnerv1.Task) error {
	l := log.WithField("func", "dispatchTask")
	defer func() {
		e := recover()
		if e != nil {
			l.Errorf("panic error: %v", e)
		}
	}()

	runCtx, cancel := context.WithTimeout(ctx, p.cfg.Runner.Timeout)
	defer cancel()

	return p.Dispatch(runCtx, task)
}
