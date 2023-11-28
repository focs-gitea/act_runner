// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"github.com/bufbuild/connect-go"
	"github.com/kardianos/service"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"gitea.com/gitea/act_runner/internal/app/poll"
	"gitea.com/gitea/act_runner/internal/app/run"
	"gitea.com/gitea/act_runner/internal/pkg/client"
	"gitea.com/gitea/act_runner/internal/pkg/config"
	"gitea.com/gitea/act_runner/internal/pkg/envcheck"
	"gitea.com/gitea/act_runner/internal/pkg/labels"
	"gitea.com/gitea/act_runner/internal/pkg/ver"
)

func daemon(cmd *cobra.Command, ctx context.Context, configFile *string, directory *string, daemonUser *string) error {
	svc, err := svc(ctx, configFile, directory, daemonUser)
	if err != nil {
		return err
	}
	serviceAction := cmd.Use
	switch serviceAction {
	case "install":
		if err := svc.Install(); err != nil {
			return err
		}
		return svc.Start()
	case "uninstall":
		if err := svc.Stop(); err != nil {
			log.Println(err)
		}
		return svc.Uninstall()
	default:
		return svc.Run()
	}
}

func svc(ctx context.Context, configFile *string, directory *string, daemonUser *string) (service.Service, error) {
	/*
	 * The following struct fields are used to set the service's Name,
	 * Display name, description and Arguments only when service  gets
	 * installed via the `admin-helper install-daemon` command for CRC
	 * in production these values are not used as the MSI installs the
	 * service
	 */
	svcConfig := &service.Config{
		Name:        "act-runner",
		DisplayName: "Gitea-CI runner",
		Description: "Gitea CI runner written in GO",
		Arguments:   []string{"daemon"},
	}
	svcConfig.Arguments = append(svcConfig.Arguments, "--working-directory", *directory)

	if *configFile != "" {
		svcConfig.Arguments = append(svcConfig.Arguments, "--config", *configFile)
	} else {
		configFile := filepath.Join(*directory, "config.yaml")
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			file, err := os.Create(configFile)
			if err != nil {
				log.Error("Error creating config.yaml:", err)
				os.Exit(1)
			}
			defer file.Close()
			_, err = file.Write(config.Example)
			if err != nil {
				log.Error("Error writing to config.yaml:", err)
				os.Exit(1)
			}
		} else if err != nil {
			log.Error("Error checking config.yaml:", err)
			os.Exit(1)
		}

		svcConfig.Arguments = append(svcConfig.Arguments, "--config", configFile)
	}

	if runtime.GOOS == "linux" {
		if os.Getuid() != 0 {
			log.Fatal("The --user is not supported for non-root users")
		}
		if *daemonUser != "" {
			svcConfig.UserName = *daemonUser
		}
	}

	if runtime.GOOS == "darwin" {
		svcConfig.EnvVars = map[string]string{
			"PATH": "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/bin:/bin:/usr/sbin:/sbin",
		}
		svcConfig.Option = service.KeyValue{
			"KeepAlive":   true,
			"RunAtLoad":   true,
			"UserService": os.Getuid() != 0,
		}
	}

	prg := &program{
		ctx:              ctx,
		configFile:       configFile,
		workingDirectory: directory,
	}
	return service.New(prg, svcConfig)
}

type program struct {
	ctx              context.Context
	configFile       *string
	workingDirectory *string

	// stopSignals is to catch a signals notified to process: SIGTERM, SIGQUIT, Interrupt, Kill
	stopSignals chan os.Signal

	// done channel to signal the completion of the run method
	done chan struct{}
}

func (p *program) Start(s service.Service) error {
	p.stopSignals = make(chan os.Signal)
	p.done = make(chan struct{})

	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}
func (p *program) Stop(s service.Service) error {
	close(p.stopSignals)
	<-p.done // Wait for the run method to complete

	// Stop should not block. Return with a few seconds.
	return nil
}

func (p *program) run() {
	signal.Notify(p.stopSignals, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt)
	// Do work here
	if err := os.Chdir(*p.workingDirectory); err != nil {
		log.Error("error changing working directory:", err)
		os.Exit(1)
	}

	cfg, err := config.LoadDefault(*p.configFile)
	if err != nil {
		log.Error("invalid configuration: %w", err)
		os.Exit(1)
	}
	initLogging(cfg)
	log.Infoln("Starting runner daemon")

	reg, err := config.LoadRegistration(cfg.Runner.File)
	if os.IsNotExist(err) {
		log.Error("registration file not found, please register the runner first")
		os.Exit(1)
	} else if err != nil {
		log.Error("failed to load registration file: %w", err)
		os.Exit(1)
	}

	lbls := reg.Labels
	if len(cfg.Runner.Labels) > 0 {
		lbls = cfg.Runner.Labels
	}

	ls := labels.Labels{}
	for _, l := range lbls {
		label, err := labels.Parse(l)
		if err != nil {
			log.WithError(err).Warnf("ignored invalid label %q", l)
			continue
		}
		ls = append(ls, label)
	}
	if len(ls) == 0 {
		log.Warn("no labels configured, runner may not be able to pick up jobs")
	}

	if ls.RequireDocker() {
		dockerSocketPath, err := getDockerSocketPath(cfg.Container.DockerHost)
		if err != nil {
			log.Error(err)
			return
		}
		if err := envcheck.CheckIfDockerRunning(p.ctx, dockerSocketPath); err != nil {
			log.Error(err)
			return
		}
		// if dockerSocketPath passes the check, override DOCKER_HOST with dockerSocketPath
		os.Setenv("DOCKER_HOST", dockerSocketPath)
		// empty cfg.Container.DockerHost means act_runner need to find an available docker host automatically
		// and assign the path to cfg.Container.DockerHost
		if cfg.Container.DockerHost == "" {
			cfg.Container.DockerHost = dockerSocketPath
		}
		// check the scheme, if the scheme is not npipe or unix
		// set cfg.Container.DockerHost to "-" because it can't be mounted to the job container
		if protoIndex := strings.Index(cfg.Container.DockerHost, "://"); protoIndex != -1 {
			scheme := cfg.Container.DockerHost[:protoIndex]
			if !strings.EqualFold(scheme, "npipe") && !strings.EqualFold(scheme, "unix") {
				cfg.Container.DockerHost = "-"
			}
		}
	}

	cli := client.New(
		reg.Address,
		cfg.Runner.Insecure,
		reg.UUID,
		reg.Token,
		ver.Version(),
	)

	runner := run.NewRunner(cfg, reg, cli)
	// declare the labels of the runner before fetching tasks
	resp, err := runner.Declare(p.ctx, ls.Names())
	if err != nil && connect.CodeOf(err) == connect.CodeUnimplemented {
		// Gitea instance is older version. skip declare step.
		log.Warn("Because the Gitea instance is an old version, skip declare labels and version.")
	} else if err != nil {
		log.WithError(err).Error("fail to invoke Declare")
		return
	} else {
		log.Infof("runner: %s, with version: %s, with labels: %v, declare successfully",
			resp.Msg.Runner.Name, resp.Msg.Runner.Version, resp.Msg.Runner.Labels)
		// if declare successfully, override the labels in the.runner file with valid labels in the config file (if specified)
		reg.Labels = ls.ToStrings()
		if err := config.SaveRegistration(cfg.Runner.File, reg); err != nil {
			log.Error("failed to save runner config: %w", err)
			return
		}
	}

	poller := poll.New(cfg, cli, runner)
	poller.Poll(p.ctx)

	close(p.done) // Signal that the run method has completed

	return
}

// initLogging setup the global logrus logger.
func initLogging(cfg *config.Config) {
	isTerm := isatty.IsTerminal(os.Stdout.Fd())
	format := &log.TextFormatter{
		DisableColors: !isTerm,
		FullTimestamp: true,
	}
	log.SetFormatter(format)

	if l := cfg.Log.Level; l != "" {
		level, err := log.ParseLevel(l)
		if err != nil {
			log.WithError(err).
				Errorf("invalid log level: %q", l)
		}

		// debug level
		if level == log.DebugLevel {
			log.SetReportCaller(true)
			format.CallerPrettyfier = func(f *runtime.Frame) (string, string) {
				// get function name
				s := strings.Split(f.Function, ".")
				funcname := "[" + s[len(s)-1] + "]"
				// get file name and line number
				_, filename := path.Split(f.File)
				filename = "[" + filename + ":" + strconv.Itoa(f.Line) + "]"
				return funcname, filename
			}
			log.SetFormatter(format)
		}

		if log.GetLevel() != level {
			log.Infof("log level changed to %v", level)
			log.SetLevel(level)
		}
	}
}

var commonSocketPaths = []string{
	"/var/run/docker.sock",
	"/run/podman/podman.sock",
	"$HOME/.colima/docker.sock",
	"$XDG_RUNTIME_DIR/docker.sock",
	"$XDG_RUNTIME_DIR/podman/podman.sock",
	`\\.\pipe\docker_engine`,
	"$HOME/.docker/run/docker.sock",
}

func getDockerSocketPath(configDockerHost string) (string, error) {
	// a `-` means don't mount the docker socket to job containers
	if configDockerHost != "" && configDockerHost != "-" {
		return configDockerHost, nil
	}

	socket, found := os.LookupEnv("DOCKER_HOST")
	if found {
		return socket, nil
	}

	for _, p := range commonSocketPaths {
		if _, err := os.Lstat(os.ExpandEnv(p)); err == nil {
			if strings.HasPrefix(p, `\\.\`) {
				return "npipe://" + filepath.ToSlash(os.ExpandEnv(p)), nil
			}
			return "unix://" + filepath.ToSlash(os.ExpandEnv(p)), nil
		}
	}

	return "", fmt.Errorf("daemon Docker Engine socket not found and docker_host config was invalid")
}
