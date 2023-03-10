// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.5"

var RunnerVersion = "develop"

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

	// ./act_runner version
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of act runner",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("act_runner version %s\n", RunnerVersion)
		},
	}
	rootCmd.AddCommand(versionCmd)

	// ./act_runner daemon
	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run as a runner daemon",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDaemon(ctx, gArgs.EnvFile),
	}
	rootCmd.AddCommand(daemonCmd)

	// ./act_runner exec
	rootCmd.AddCommand(loadExecCmd(ctx))

	// hide completion command
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
