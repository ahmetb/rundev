package main

import (
	"bytes"
	"context"
	"flag"
	"github.com/google/shlex"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

var (
	flLocalDir   *string
	flRemoteDir  *string
	flAddr       *string
	flBuildCmd   *string
	flRunCmd     *string
	flNoCloudRun *bool
)

type remoteRunOpts struct {
	syncDir  string
	buildCmd []string
	runCmd   []string
}

func init() {
	flLocalDir = flag.String("local-dir", ".", "local directory to sync")
	flRemoteDir = flag.String("remote-dir", "", "remote directory to sync (inside the container), defaults to container's WORKDIR")
	flAddr = flag.String("addr", "localhost:8080", "network address to start the local proxy server")
	flBuildCmd = flag.String("build-cmd", "", "(optional) command to re-build code (inside the container) after syncing")
	flRunCmd = flag.String("run-cmd", "", "(optional) command to start application (inside the container) after syncing, inferred from Dockerfile by default")
	flNoCloudRun = flag.Bool("no-cloudrun", false, "do not deploy to Cloud Run (you should start rundevd on localhost:8888)")
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
	if fi, err := os.Stat(*flLocalDir); err != nil {
		log.Fatalf("cannot open -local-dir: %v", err)
	} else if !fi.IsDir() {
		log.Fatalf("-local-dir (%s) is not a directory (%s)", *flLocalDir, fi.Mode())
	}

	var rundevdURL string
	if *flNoCloudRun {
		rundevdURL = "http://localhost:8888"
		log.Printf("not deploying to Cloud Run. make sure to start rundevd at %s", rundevdURL)
	} else {
		log.Printf("starting one-time build & push & deploy")
		const appName = `rundev-app` // TODO(ahmetb) use basename(realpath($CWD)
		project, err := currentProject(ctx)
		if err != nil {
			log.Fatalf("error reading current project ID from gcloud: %+v", err)
		}
		imageName := `gcr.io/` + project + `/` + appName

		df, err := readDockerfile(*flLocalDir)
		if err != nil {
			log.Fatal(err)
		}

		var runCmd, buildCmd cmd
		if *flRunCmd == "" {
			runCmd, err = parseDockerfileEntrypoint(df)
			if err != nil {
				log.Fatalf("failed to parse entrypoint/cmd from dockerfile. try specifying -run-cmd? error: %+v", err)
			}
			log.Printf("[info] parsed entrypoint as %s", runCmd)
		} else {
			v, err := shlex.Split(*flRunCmd)
			if err != nil {
				log.Fatalf("failed to parse -run-cmd into commands and args: %+v", err)
			}
			runCmd = cmd{v[0], v[1:]}
		}

		if *flBuildCmd == "" {
			log.Printf("[info] -build-cmd not specified: if you have steps to build your code after syncing, use this flag")
		} else {
			v, err := shlex.Split(*flBuildCmd)
			if err != nil {
				log.Fatalf("failed to parse -build-cmd into commands and args: %+v", err)
			}
			buildCmd = cmd{v[0], v[1:]}
			log.Printf("[info] parsed -build-cmd as: %s", buildCmd)
		}

		ro := remoteRunOpts{
			syncDir:  *flRemoteDir,
			runCmd:   runCmd.List(),
			buildCmd: buildCmd.List(),
		}
		newEntrypoint := prepEntrypoint(ro)
		log.Printf("[info] injecting to dockerfile:\n%s", regexp.MustCompile("(?m)^").ReplaceAllString(newEntrypoint, "\t"))

		df = append(df, '\n')
		df = append(df, []byte(newEntrypoint)...)
		bo := buildOpts{
			dir:        *flLocalDir,
			image:      imageName,
			dockerfile: df}
		log.Print("building and pushing docker image")
		if err := dockerBuildPush(ctx, bo); err != nil {
			log.Fatal(err)
		}
		log.Printf("built and pushed docker image: %s", imageName)

		log.Print("deploying to Cloud Run")
		appURL, err := deployCloudRun(ctx, appName, project, imageName)
		if err != nil {
			log.Fatalf("error deploying to Cloud Run: %+v", err)
		}
		rundevdURL = appURL
	}
	sync := newSyncer(syncOpts{
		localDir:   *flLocalDir,
		targetAddr: rundevdURL,
	})
	localServerHandler, err := newLocalServer(localServerOpts{
		proxyTarget: rundevdURL,
		sync:        sync,
	})
	if err != nil {
		log.Fatalf("failed to initialize local server: %+v", err)
	}
	localServer := http.Server{
		Handler: localServerHandler,
		Addr:    *flAddr}

	go func() {
		<-ctx.Done()
		log.Println("shutting down server")
		localServer.Shutdown(ctx) // TODO(ahmetb) maybe use .Close?
	}()
	log.Printf("local proxy server starting at %s (proxying to %s)", *flAddr, rundevdURL)
	if err := localServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("local server shut down gracefully, exiting")
			os.Exit(0)
		}
		log.Fatalf("local server failed to start: %+v", err)
	}
}

func deployCloudRun(ctx context.Context, appName, project, image string) (string, error) {
	b, err := exec.CommandContext(ctx, "gcloud",
		"beta", "run", "deploy", "-q", appName,
		"--image="+image,
		"--project="+project,
		"--platform=gke",
		"--cluster=cloudrun",
		"--cluster-location=us-central1",
	).CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "cloud run deployment failed. output:\n%s", string(b))
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gcloud", "beta",
		"run", "services", "describe", "-q", appName,
		"--project="+project,
		"--platform=gke",
		"--cluster=cloudrun",
		"--cluster-location=us-central1",
		"--format=get(status.url)")
	cmd.Stderr = &stderr
	b, err = cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "cloud run describe failed. stderr:\n%s", string(stderr.Bytes()))
	}
	return strings.TrimSpace(string(b)), nil
}

func currentProject(ctx context.Context) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gcloud", "config", "get-value", "core/project", "-q")
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	return strings.TrimRightFunc(string(b), unicode.IsSpace),
		errors.Wrapf(err, "failed to read current GCP project from gcloud: output=%q", stderr.String())
}
