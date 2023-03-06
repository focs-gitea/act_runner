// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/nektos/act/pkg/artifacts"
	"github.com/nektos/act/pkg/common"
	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type executeArgs struct {
	runList               bool
	job                   string
	event                 string
	workdir               string
	workflowsPath         string
	noWorkflowRecurse     bool
	autodetectEvent       bool
	forcePull             bool
	forceRebuild          bool
	jsonLogger            bool
	envs                  []string
	envfile               string
	secrets               []string
	defaultActionsUrl     string
	insecureSecrets       bool
	privileged            bool
	usernsMode            string
	containerArchitecture string
	containerDaemonSocket string
	useGitIgnore          bool
	containerCapAdd       []string
	containerCapDrop      []string
	artifactServerPath    string
	artifactServerPort    string
	noSkipCheckout        bool
	debug                 bool
	dryrun                bool
}

// WorkflowsPath returns path to workflow file(s)
func (i *executeArgs) WorkflowsPath() string {
	return i.resolve(i.workflowsPath)
}

// Envfile returns path to .env
func (i *executeArgs) Envfile() string {
	return i.resolve(i.envfile)
}

func (i *executeArgs) LoadSecrets() map[string]string {
	s := make(map[string]string)
	for _, secretPair := range i.secrets {
		secretPairParts := strings.SplitN(secretPair, "=", 2)
		secretPairParts[0] = strings.ToUpper(secretPairParts[0])
		if strings.ToUpper(s[secretPairParts[0]]) == secretPairParts[0] {
			log.Errorf("Secret %s is already defined (secrets are case insensitive)", secretPairParts[0])
		}
		if len(secretPairParts) == 2 {
			s[secretPairParts[0]] = secretPairParts[1]
		} else if env, ok := os.LookupEnv(secretPairParts[0]); ok && env != "" {
			s[secretPairParts[0]] = env
		} else {
			fmt.Printf("Provide value for '%s': ", secretPairParts[0])
			val, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				log.Errorf("failed to read input: %v", err)
				os.Exit(1)
			}
			s[secretPairParts[0]] = string(val)
		}
	}
	return s
}

func readEnvs(path string, envs map[string]string) bool {
	if _, err := os.Stat(path); err == nil {
		env, err := godotenv.Read(path)
		if err != nil {
			log.Fatalf("Error loading from %s: %v", path, err)
		}
		for k, v := range env {
			envs[k] = v
		}
		return true
	}
	return false
}

func (i *executeArgs) LoadEnvs() map[string]string {
	envs := make(map[string]string)
	if i.envs != nil {
		for _, envVar := range i.envs {
			e := strings.SplitN(envVar, `=`, 2)
			if len(e) == 2 {
				envs[e[0]] = e[1]
			} else {
				envs[e[0]] = ""
			}
		}
	}
	_ = readEnvs(i.Envfile(), envs)

	return envs
}

// Workdir returns path to workdir
func (i *executeArgs) Workdir() string {
	return i.resolve(".")
}

func (i *executeArgs) resolve(path string) string {
	basedir, err := filepath.Abs(i.workdir)
	if err != nil {
		log.Fatal(err)
	}
	if path == "" {
		return path
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(basedir, path)
	}
	return path
}

func printList(plan *model.Plan) error {
	type lineInfoDef struct {
		jobID   string
		jobName string
		stage   string
		wfName  string
		wfFile  string
		events  string
	}
	lineInfos := []lineInfoDef{}

	header := lineInfoDef{
		jobID:   "Job ID",
		jobName: "Job name",
		stage:   "Stage",
		wfName:  "Workflow name",
		wfFile:  "Workflow file",
		events:  "Events",
	}

	jobs := map[string]bool{}
	duplicateJobIDs := false

	jobIDMaxWidth := len(header.jobID)
	jobNameMaxWidth := len(header.jobName)
	stageMaxWidth := len(header.stage)
	wfNameMaxWidth := len(header.wfName)
	wfFileMaxWidth := len(header.wfFile)
	eventsMaxWidth := len(header.events)

	for i, stage := range plan.Stages {
		for _, r := range stage.Runs {
			jobID := r.JobID
			line := lineInfoDef{
				jobID:   jobID,
				jobName: r.String(),
				stage:   strconv.Itoa(i),
				wfName:  r.Workflow.Name,
				wfFile:  r.Workflow.File,
				events:  strings.Join(r.Workflow.On(), `,`),
			}
			if _, ok := jobs[jobID]; ok {
				duplicateJobIDs = true
			} else {
				jobs[jobID] = true
			}
			lineInfos = append(lineInfos, line)
			if jobIDMaxWidth < len(line.jobID) {
				jobIDMaxWidth = len(line.jobID)
			}
			if jobNameMaxWidth < len(line.jobName) {
				jobNameMaxWidth = len(line.jobName)
			}
			if stageMaxWidth < len(line.stage) {
				stageMaxWidth = len(line.stage)
			}
			if wfNameMaxWidth < len(line.wfName) {
				wfNameMaxWidth = len(line.wfName)
			}
			if wfFileMaxWidth < len(line.wfFile) {
				wfFileMaxWidth = len(line.wfFile)
			}
			if eventsMaxWidth < len(line.events) {
				eventsMaxWidth = len(line.events)
			}
		}
	}

	jobIDMaxWidth += 2
	jobNameMaxWidth += 2
	stageMaxWidth += 2
	wfNameMaxWidth += 2
	wfFileMaxWidth += 2

	fmt.Printf("%*s%*s%*s%*s%*s%*s\n",
		-stageMaxWidth, header.stage,
		-jobIDMaxWidth, header.jobID,
		-jobNameMaxWidth, header.jobName,
		-wfNameMaxWidth, header.wfName,
		-wfFileMaxWidth, header.wfFile,
		-eventsMaxWidth, header.events,
	)
	for _, line := range lineInfos {
		fmt.Printf("%*s%*s%*s%*s%*s%*s\n",
			-stageMaxWidth, line.stage,
			-jobIDMaxWidth, line.jobID,
			-jobNameMaxWidth, line.jobName,
			-wfNameMaxWidth, line.wfName,
			-wfFileMaxWidth, line.wfFile,
			-eventsMaxWidth, line.events,
		)
	}
	if duplicateJobIDs {
		fmt.Print("\nDetected multiple jobs with the same job name, use `-W` to specify the path to the specific workflow.\n")
	}
	return nil
}

func runExecList(ctx context.Context, planner model.WorkflowPlanner, execArgs *executeArgs) error {
	// plan with filtered jobs - to be used for filtering only
	var filterPlan *model.Plan

	// Determine the event name to be filtered
	var filterEventName string = ""

	if len(execArgs.event) > 0 {
		log.Infof("Using chosed event for filtering: %s", execArgs.event)
		filterEventName = execArgs.event
	} else if execArgs.autodetectEvent {
		// collect all events from loaded workflows
		events := planner.GetEvents()

		// set default event type to first event from many available
		// this way user dont have to specify the event.
		log.Infof("Using first detected workflow event for filtering: %s", events[0])

		filterEventName = events[0]
	}

	if execArgs.job != "" {
		log.Infof("Preparing plan with a job: %s", execArgs.job)
		filterPlan = planner.PlanJob(execArgs.job)
	} else if filterEventName != "" {
		log.Infof("Preparing plan for a event: %s", filterEventName)
		filterPlan = planner.PlanEvent(filterEventName)
	} else {
		log.Infof("Preparing plan with all jobs")
		filterPlan = planner.PlanAll()
	}

	printList(filterPlan)

	return nil
}

func runExec(ctx context.Context, execArgs *executeArgs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		planner, err := model.NewWorkflowPlanner(execArgs.WorkflowsPath(), execArgs.noWorkflowRecurse)
		if err != nil {
			return err
		}

		if execArgs.runList {
			return runExecList(ctx, planner, execArgs)
		}

		// plan with triggered jobs
		var plan *model.Plan

		// Determine the event name to be triggered
		var eventName string

		// collect all events from loaded workflows
		events := planner.GetEvents()

		if len(execArgs.event) > 0 {
			log.Infof("Using chosed event for filtering: %s", execArgs.event)
			eventName = args[0]
		} else if len(events) == 1 && len(events[0]) > 0 {
			log.Infof("Using the only detected workflow event: %s", events[0])
			eventName = events[0]
		} else if execArgs.autodetectEvent && len(events) > 0 && len(events[0]) > 0 {
			// set default event type to first event from many available
			// this way user dont have to specify the event.
			log.Infof("Using first detected workflow event: %s", events[0])
			eventName = events[0]
		} else {
			log.Infof("Using default workflow event: push")
			eventName = "push"
		}

		// build the plan for this run
		if execArgs.job != "" {
			log.Infof("Planning job: %s", execArgs.job)
			plan = planner.PlanJob(execArgs.job)
		} else {
			log.Infof("Planning jobs for event: %s", eventName)
			plan = planner.PlanEvent(eventName)
		}

		maxLifetime := 3 * time.Hour
		if deadline, ok := ctx.Deadline(); ok {
			maxLifetime = time.Until(deadline)
		}

		// run the plan
		config := &runner.Config{
			Workdir:               execArgs.Workdir(),
			BindWorkdir:           false,
			ReuseContainers:       false,
			ForcePull:             execArgs.forcePull,
			ForceRebuild:          execArgs.forceRebuild,
			LogOutput:             true,
			JSONLogger:            execArgs.jsonLogger,
			Env:                   execArgs.LoadEnvs(),
			Secrets:               execArgs.LoadSecrets(),
			InsecureSecrets:       execArgs.insecureSecrets,
			Privileged:            execArgs.privileged,
			UsernsMode:            execArgs.usernsMode,
			ContainerArchitecture: execArgs.containerArchitecture,
			ContainerDaemonSocket: execArgs.containerDaemonSocket,
			UseGitIgnore:          execArgs.useGitIgnore,
			// GitHubInstance:        t.client.Address(),
			ContainerCapAdd:    execArgs.containerCapAdd,
			ContainerCapDrop:   execArgs.containerCapDrop,
			AutoRemove:         true,
			ArtifactServerPath: execArgs.artifactServerPath,
			ArtifactServerPort: execArgs.artifactServerPort,
			NoSkipCheckout:     execArgs.noSkipCheckout,
			// PresetGitHubContext:   preset,
			// EventJSON:             string(eventJSON),
			ContainerNamePrefix:   fmt.Sprintf("GITEA-ACTIONS-TASK-%s", eventName),
			ContainerMaxLifetime:  maxLifetime,
			ContainerNetworkMode:  "bridge",
			DefaultActionInstance: execArgs.defaultActionsUrl,
			PlatformPicker: func(_ []string) string {
				return "node:16-bullseye"
			},
		}

		if execArgs.debug {
			logrus.SetLevel(log.TraceLevel)
		} else {
			logrus.SetLevel(log.InfoLevel)
		}

		r, err := runner.New(config)
		if err != nil {
			return err
		}

		if len(execArgs.artifactServerPath) == 0 {
			tempDir, err := os.MkdirTemp("", "gitea-act-")
			if err != nil {
				fmt.Println(err)
			}
			defer os.RemoveAll(tempDir)

			execArgs.artifactServerPath = tempDir
		}

		artifactCancel := artifacts.Serve(ctx, execArgs.artifactServerPath, execArgs.artifactServerPort)
		log.Debugf("artifacts server started at %s:%s", execArgs.artifactServerPath, execArgs.artifactServerPort)

		ctx = common.WithDryrun(ctx, execArgs.dryrun)

		executor := r.NewPlanExecutor(plan).Finally(func(ctx context.Context) error {
			artifactCancel()
			return nil
		})

		return executor(ctx)
	}
}
