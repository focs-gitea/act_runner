// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package run

import (
	"bytes"
	"fmt"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	"github.com/nektos/act/pkg/model"
)

func generateWorkflow(task *runnerv1.Task) (*model.Workflow, string, error) {
	workflow, err := model.ReadWorkflow(bytes.NewReader(task.WorkflowPayload))
	if err != nil {
		return nil, "", err
	}

	jobIDs := workflow.GetJobIDs()
	if len(jobIDs) != 1 {
		return nil, "", fmt.Errorf("multiple jobs found: %v", jobIDs)
	}
	jobID := jobIDs[0]

	return workflow, jobID, nil
}
