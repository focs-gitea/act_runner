// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package report

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	runnerv1 "code.gitea.io/actions-proto-go/runner/v1"
	retry "github.com/avast/retry-go/v4"
	"github.com/bufbuild/connect-go"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"gitea.com/gitea/act_runner/internal/pkg/client"
)

type Reporter struct {
	ctx    context.Context
	cancel context.CancelFunc

	closed  bool
	client  client.Client
	clientM sync.Mutex

	logOffset   int
	logRows     []*runnerv1.LogRow
	logReplacer *strings.Replacer

	state       *runnerv1.TaskState
	outputs     map[string]string
	sentOutputs map[string]bool
	mutex       sync.RWMutex
}

func NewReporter(ctx context.Context, cancel context.CancelFunc, client client.Client, task *runnerv1.Task) *Reporter {
	var oldnew []string
	if v := task.Context.Fields["token"].GetStringValue(); v != "" {
		oldnew = append(oldnew, v, "***")
	}
	for _, v := range task.Secrets {
		oldnew = append(oldnew, v, "***")
	}

	return &Reporter{
		ctx:         ctx,
		cancel:      cancel,
		client:      client,
		logReplacer: strings.NewReplacer(oldnew...),
		state: &runnerv1.TaskState{
			Id: task.Id,
		},
		outputs:     map[string]string{},
		sentOutputs: map[string]bool{},
	}
}

func (r *Reporter) ResetSteps(l int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i := 0; i < l; i++ {
		r.state.Steps = append(r.state.Steps, &runnerv1.StepState{
			Id: int64(i),
		})
	}
}

func (r *Reporter) Levels() []log.Level {
	return log.AllLevels
}

func (r *Reporter) Fire(entry *log.Entry) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	log.WithFields(entry.Data).Trace(entry.Message)

	timestamp := entry.Time
	if r.state.StartedAt == nil {
		r.state.StartedAt = timestamppb.New(timestamp)
	}

	stage := entry.Data["stage"]

	if stage != "Main" {
		if v, ok := entry.Data["jobResult"]; ok {
			if jobResult, ok := r.parseResult(v); ok {
				r.state.Result = jobResult
				r.state.StoppedAt = timestamppb.New(timestamp)
				for _, s := range r.state.Steps {
					if s.Result == runnerv1.Result_RESULT_UNSPECIFIED {
						s.Result = runnerv1.Result_RESULT_CANCELLED
					}
				}
			}
		}
		if !r.duringSteps() {
			r.logRows = append(r.logRows, r.parseLogRow(entry))
		}
		return nil
	}

	var step *runnerv1.StepState
	if v, ok := entry.Data["stepNumber"]; ok {
		if v, ok := v.(int); ok && len(r.state.Steps) > v {
			step = r.state.Steps[v]
		}
	}
	if step == nil {
		if !r.duringSteps() {
			r.logRows = append(r.logRows, r.parseLogRow(entry))
		}
		return nil
	}

	if step.StartedAt == nil {
		step.StartedAt = timestamppb.New(timestamp)
	}
	if v, ok := entry.Data["raw_output"]; ok {
		if rawOutput, ok := v.(bool); ok && rawOutput {
			if step.LogLength == 0 {
				step.LogIndex = int64(r.logOffset + len(r.logRows))
			}
			step.LogLength++
			r.logRows = append(r.logRows, r.parseLogRow(entry))
		}
	} else if !r.duringSteps() {
		r.logRows = append(r.logRows, r.parseLogRow(entry))
	}
	if v, ok := entry.Data["stepResult"]; ok {
		if stepResult, ok := r.parseResult(v); ok {
			if step.LogLength == 0 {
				step.LogIndex = int64(r.logOffset + len(r.logRows))
			}
			step.Result = stepResult
			step.StoppedAt = timestamppb.New(timestamp)
		}
	}

	return nil
}

func (r *Reporter) RunDaemon() {
	if r.closed {
		return
	}
	if r.ctx.Err() != nil {
		return
	}

	_ = r.ReportLog(false)
	_ = r.ReportState()

	time.AfterFunc(time.Second, r.RunDaemon)
}

func (r *Reporter) Logf(format string, a ...interface{}) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.duringSteps() {
		r.logRows = append(r.logRows, &runnerv1.LogRow{
			Time:    timestamppb.Now(),
			Content: fmt.Sprintf(format, a...),
		})
	}
}

func (r *Reporter) SetOutputs(outputs map[string]string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for k, v := range outputs {
		r.outputs[k] = v
		r.sentOutputs[k] = false
	}
}

func (r *Reporter) Close(lastWords string) error {
	r.closed = true

	r.mutex.Lock()
	if r.state.Result == runnerv1.Result_RESULT_UNSPECIFIED {
		if lastWords == "" {
			lastWords = "Early termination"
		}
		for _, v := range r.state.Steps {
			if v.Result == runnerv1.Result_RESULT_UNSPECIFIED {
				v.Result = runnerv1.Result_RESULT_CANCELLED
			}
		}
		r.state.Result = runnerv1.Result_RESULT_FAILURE
		r.logRows = append(r.logRows, &runnerv1.LogRow{
			Time:    timestamppb.Now(),
			Content: lastWords,
		})
		r.state.StoppedAt = timestamppb.Now()
	} else if lastWords != "" {
		r.logRows = append(r.logRows, &runnerv1.LogRow{
			Time:    timestamppb.Now(),
			Content: lastWords,
		})
	}
	r.mutex.Unlock()

	return retry.Do(func() error {
		if err := r.ReportLog(true); err != nil {
			return err
		}
		return r.ReportState()
	}, retry.Context(r.ctx))
}

func (r *Reporter) ReportLog(noMore bool) error {
	r.clientM.Lock()
	defer r.clientM.Unlock()

	r.mutex.RLock()
	rows := r.logRows
	r.mutex.RUnlock()

	resp, err := r.client.UpdateLog(r.ctx, connect.NewRequest(&runnerv1.UpdateLogRequest{
		TaskId: r.state.Id,
		Index:  int64(r.logOffset),
		Rows:   rows,
		NoMore: noMore,
	}))
	if err != nil {
		return err
	}

	ack := int(resp.Msg.AckIndex)
	if ack < r.logOffset {
		return fmt.Errorf("submitted logs are lost")
	}

	r.mutex.Lock()
	r.logRows = r.logRows[ack-r.logOffset:]
	r.logOffset = ack
	r.mutex.Unlock()

	if noMore && ack < r.logOffset+len(rows) {
		return fmt.Errorf("not all logs are submitted")
	}

	return nil
}

func (r *Reporter) ReportState() error {
	r.clientM.Lock()
	defer r.clientM.Unlock()

	r.mutex.RLock()
	state := proto.Clone(r.state).(*runnerv1.TaskState)
	r.mutex.RUnlock()

	outputs := make(map[string]string, len(r.outputs))
	for k, v := range r.outputs {
		if !r.sentOutputs[k] {
			outputs[k] = v
		}
	}

	resp, err := r.client.UpdateTask(r.ctx, connect.NewRequest(&runnerv1.UpdateTaskRequest{
		State:   state,
		Outputs: outputs,
	}))
	if err != nil {
		return err
	}

	for _, k := range resp.Msg.SentOutputs {
		r.sentOutputs[k] = true
	}

	if resp.Msg.State != nil && resp.Msg.State.Result == runnerv1.Result_RESULT_CANCELLED {
		r.cancel()
	}

	return nil
}

func (r *Reporter) duringSteps() bool {
	if steps := r.state.Steps; len(steps) == 0 {
		return false
	} else if first := steps[0]; first.Result == runnerv1.Result_RESULT_UNSPECIFIED && first.LogLength == 0 {
		return false
	} else if last := steps[len(steps)-1]; last.Result != runnerv1.Result_RESULT_UNSPECIFIED {
		return false
	}
	return true
}

var stringToResult = map[string]runnerv1.Result{
	"success":   runnerv1.Result_RESULT_SUCCESS,
	"failure":   runnerv1.Result_RESULT_FAILURE,
	"skipped":   runnerv1.Result_RESULT_SKIPPED,
	"cancelled": runnerv1.Result_RESULT_CANCELLED,
}

func (r *Reporter) parseResult(result interface{}) (runnerv1.Result, bool) {
	str := ""
	if v, ok := result.(string); ok { // for jobResult
		str = v
	} else if v, ok := result.(fmt.Stringer); ok { // for stepResult
		str = v.String()
	}

	ret, ok := stringToResult[str]
	return ret, ok
}

func (r *Reporter) parseLogRow(entry *log.Entry) *runnerv1.LogRow {
	content := strings.TrimRightFunc(entry.Message, func(r rune) bool { return r == '\r' || r == '\n' })
	content = r.logReplacer.Replace(content)
	return &runnerv1.LogRow{
		Time:    timestamppb.New(entry.Time),
		Content: content,
	}
}
