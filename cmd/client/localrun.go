package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/pkg/errors"
)

type localRunSession struct {
	containerImage string
	containerName  string
	localPort      int

	stderr bytes.Buffer
	cmd    *exec.Cmd
}

func (s *localRunSession) start(ctx context.Context) error {
	s.cmd = exec.CommandContext(ctx, "docker", "run", "--rm",
		"--name="+s.containerName,
		"--env=PORT=8080",
		fmt.Sprintf("--publish=%d:8080", s.localPort),
		s.containerImage)
	s.cmd.Stderr = &s.stderr
	err := s.cmd.Start()
	return errors.Wrap(err, "failed to start local docker run session")
}

func (s *localRunSession) addr() string {
	return fmt.Sprintf("http://localhost:%d/", s.localPort)
}

func (s *localRunSession) wait(ctx context.Context) error {
	stopCh := make(chan error, 1)
	defer close(stopCh)
	go func() { stopCh <- s.cmd.Wait() }()

	select {
	// no need to add <-ctx.Done() case here as command is started with CommandContext
	case err := <-stopCh:
		log.Printf("docker run [stderr:] %s", s.stderr.String())
		if err != nil {
			errors.Wrap(err, "local container unexpectedly exited")
		}
		return errors.New("container unexpectedly exited with no error")
	}
}

func (s *localRunSession) stop(ctx context.Context) error {
	return s.cmd.Process.Kill()
}
