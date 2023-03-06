// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.5"

type globalArgs struct {
	EnvFile string
}

func Execute(ctx context.Context) {
	// task := runtime.NewTask("gitea", 0, nil, nil)

	var gArgs globalArgs

	// ./act_runner
	rootCmd := &cobra.Command{
		Use:          "act [event name to run]\nIf no event name passed, will default to \"on: push\"",
		Short:        "Run GitHub actions locally by specifying the event name (e.g. `push`) or an action name directly.",
		Args:         cobra.MaximumNArgs(1),
		Version:      version,
		SilenceUsage: true,
	}
	rootCmd.PersistentFlags().StringVarP(&gArgs.EnvFile, "env-file", "", ".env", "Read in a file of environment variables.")

	// ./act_runner register
	var regArgs registerArgs
	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register a runner to the server",
		Args:  cobra.MaximumNArgs(0),
		RunE:  runRegister(ctx, &regArgs, gArgs.EnvFile), // must use a pointer to regArgs
	}
	registerCmd.Flags().BoolVar(&regArgs.NoInteractive, "no-interactive", false, "Disable interactive mode")
	registerCmd.Flags().StringVar(&regArgs.InstanceAddr, "instance", "", "Gitea instance address")
	registerCmd.Flags().BoolVar(&regArgs.Insecure, "insecure", false, "If check server's certificate if it's https protocol")
	registerCmd.Flags().StringVar(&regArgs.Token, "token", "", "Runner token")
	registerCmd.Flags().StringVar(&regArgs.RunnerName, "name", "", "Runner name")
	registerCmd.Flags().StringVar(&regArgs.Labels, "labels", "", "Runner tags, comma separated")
	rootCmd.AddCommand(registerCmd)

	// ./act_runner daemon
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run as a runner daemon",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDaemon(ctx, gArgs.EnvFile),
	}
	rootCmd.AddCommand(daemonCmd)

	// ./act_runner exec
	execArg := executeArgs{}
	execCmd := &cobra.Command{
		Use:   "exec",
		Short: "Run workflow locally.",
		Args:  cobra.MaximumNArgs(20),
		RunE:  runExec(ctx, &execArg),
	}
	execCmd.Flags().BoolVarP(&execArg.runList, "list", "l", false, "list workflows")
	execCmd.Flags().StringVarP(&execArg.job, "job", "j", "", "run a specific job ID")
	execCmd.Flags().StringVarP(&execArg.event, "event", "E", "", "run a event name")
	execCmd.PersistentFlags().StringVarP(&execArg.workflowsPath, "workflows", "W", "./.gitea/workflows/", "path to workflow file(s)")
	execCmd.PersistentFlags().StringVarP(&execArg.workdir, "directory", "C", ".", "working directory")
	execCmd.PersistentFlags().BoolVarP(&execArg.noWorkflowRecurse, "no-recurse", "", false, "Flag to disable running workflows from subdirectories of specified path in '--workflows'/'-W' flag")
	execCmd.Flags().BoolVarP(&execArg.autodetectEvent, "detect-event", "", false, "Use first event type from workflow as event that triggered the workflow")
	execCmd.Flags().BoolVarP(&execArg.forcePull, "pull", "p", false, "pull docker image(s) even if already present")
	execCmd.Flags().BoolVarP(&execArg.forceRebuild, "rebuild", "", false, "rebuild local action docker image(s) even if already present")
	execCmd.PersistentFlags().BoolVar(&execArg.jsonLogger, "json", false, "Output logs in json format")
	execCmd.Flags().StringArrayVarP(&execArg.envs, "env", "", []string{}, "env to make available to actions with optional value (e.g. --env myenv=foo or --env myenv)")
	execCmd.PersistentFlags().StringVarP(&execArg.envfile, "env-file", "", ".env", "environment file to read and use as env in the containers")
	execCmd.Flags().StringArrayVarP(&execArg.secrets, "secret", "s", []string{}, "secret to make available to actions with optional value (e.g. -s mysecret=foo or -s mysecret)")
	execCmd.PersistentFlags().BoolVarP(&execArg.insecureSecrets, "insecure-secrets", "", false, "NOT RECOMMENDED! Doesn't hide secrets while printing logs.")
	execCmd.Flags().BoolVar(&execArg.privileged, "privileged", false, "use privileged mode")
	execCmd.Flags().StringVar(&execArg.usernsMode, "userns", "", "user namespace to use")
	execCmd.PersistentFlags().StringVarP(&execArg.containerArchitecture, "container-architecture", "", "", "Architecture which should be used to run containers, e.g.: linux/amd64. If not specified, will use host default architecture. Requires Docker server API Version 1.41+. Ignored on earlier Docker server platforms.")
	execCmd.PersistentFlags().StringVarP(&execArg.containerDaemonSocket, "container-daemon-socket", "", "/var/run/docker.sock", "Path to Docker daemon socket which will be mounted to containers")
	execCmd.Flags().BoolVar(&execArg.useGitIgnore, "use-gitignore", true, "Controls whether paths specified in .gitignore should be copied into container")
	execCmd.Flags().StringArrayVarP(&execArg.containerCapAdd, "container-cap-add", "", []string{}, "kernel capabilities to add to the workflow containers (e.g. --container-cap-add SYS_PTRACE)")
	execCmd.Flags().StringArrayVarP(&execArg.containerCapDrop, "container-cap-drop", "", []string{}, "kernel capabilities to remove from the workflow containers (e.g. --container-cap-drop SYS_PTRACE)")
	execCmd.PersistentFlags().StringVarP(&execArg.artifactServerPath, "artifact-server-path", "", ".", "Defines the path where the artifact server stores uploads and retrieves downloads from. If not specified the artifact server will not start.")
	execCmd.PersistentFlags().StringVarP(&execArg.artifactServerPort, "artifact-server-port", "", "34567", "Defines the port where the artifact server listens (will only bind to localhost).")
	execCmd.PersistentFlags().StringVarP(&execArg.defaultActionsUrl, "default-actions-url", "", "https://gitea.com", "Defines the default url of action instance.")
	execCmd.PersistentFlags().BoolVarP(&execArg.noSkipCheckout, "no-skip-checkout", "", false, "Do not skip actions/checkout")
	execCmd.PersistentFlags().BoolVarP(&execArg.debug, "debug", "d", false, "enable debug log")
	execCmd.PersistentFlags().BoolVarP(&execArg.dryrun, "dryrun", "n", false, "dryrun mode")
	rootCmd.AddCommand(execCmd)

	// hide completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
