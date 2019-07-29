package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

var (
	flLocalDir  *string
	flRemoteDir *string
	flAddr      *string
	flBuildCmd  *string
	flRunCmd    *string
)

type remoteRunOpts struct {
	dir      string
	buildCmd string
	runCmd   string
}

func init() {
	flLocalDir = flag.String("local-dir", ".", "local directory to sync")
	flAddr = flag.String("addr", ":8080", "local network address to start the local daemon")
	flRemoteDir = flag.String("remote-dir", "", "remote directory to sync (inside the container), defaults to container's WORKDIR")
	flBuildCmd = flag.String("build-cmd", "", "command to rebuild code (inside the container)")
	flRunCmd = flag.String("run-cmd", "", "command to start application (inside the container)")
	flag.Parse()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		sig := <-signalCh
		log.Printf("termination signal received: %s", sig)
		cancel()
	}()

	const project = `ahmetb-samples-playground` // TODO(ahmetb) use currentProject()
	const appName = `foo`                       // TODO(ahmetb) use basename(realpath($CWD))

	imageName := `gcr.io/` + project + `/` + appName

	df, err := readDockerfile(*flLocalDir)
	if err != nil {
		log.Fatal(err)
	}
	//ro := remoteRunOpts{
	//	dir:      *flRemoteDir,
	//	buildCmd: *flBuildCmd,
	//	runCmd:   *flRunCmd,
	//}
	//df = append(df, '\n')
	//df = append(df, []byte(prepEntrypoint(ro))...)
	//df = append(df, []byte("\nCMD []")...)
	//log.Printf("Dockerfile:\n%s", string(df))

	bo := buildOpts{
		dir:        *flLocalDir,
		image:      imageName,
		dockerfile: df}

	if err := dockerBuild(ctx, bo); err != nil {
		log.Fatal(err)
	}

	localRun := &localRunSession{
		containerImage: imageName,
		containerName:  "rundev-local",
		localPort:      5555,
	}
	if err := localRun.start(ctx); err != nil {
		log.Fatal(err)
	}
	if err := localRun.wait(ctx); err != nil {
		log.Fatal(err)
	}
}

func currentProject(ctx context.Context) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gcloud", "config", "get-value", "core/project", "-q")
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	return strings.TrimRightFunc(string(b), unicode.IsSpace),
		errors.Wrapf(err, "failed to read current GCP project from gcloud: output=%q", stderr.String())
}
