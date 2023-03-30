// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package register

import (
	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"context"
	"gitea.com/gitea/act_runner/client"
	"gitea.com/gitea/act_runner/config"
	"gitea.com/gitea/act_runner/runtime"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
)

func New(cli client.Client) *Register {
	return &Register{
		Client: cli,
	}
}

type Register struct {
	Client client.Client
}

func (p *Register) Register(ctx context.Context, reg *config.Registration) error {
	labels := make([]string, len(reg.Labels))
	for i, v := range reg.Labels {
		l, _, _, _ := runtime.ParseLabel(v)
		labels[i] = l
	}
	// register new runner.
	resp, err := p.Client.Register(ctx, connect.NewRequest(&runnerv1.RegisterRequest{
		Name:        reg.Name,
		Token:       reg.Token,
		AgentLabels: labels,
	}))
	if err != nil {
		log.WithError(err).Error("poller: cannot register new runner")
		return err
	}

	reg.ID = resp.Msg.Runner.Id
	reg.UUID = resp.Msg.Runner.Uuid
	reg.Name = resp.Msg.Runner.Name
	reg.Token = resp.Msg.Runner.Token

	return nil
}
