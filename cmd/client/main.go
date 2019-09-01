// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"github.com/ahmetb/rundev/lib/ignore"
	"github.com/ahmetb/rundev/lib/types"
	"github.com/google/shlex"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"time"
)

var (
	flLocalDir   *string
	flRemoteDir  *string
	flAddr       *string
	flBuildCmd   *string
	flRunCmd     *string
	flNoCloudRun *bool

	flCloudRunName            *string
	flCloudRunCluster         *string
	flCloudRunClusterLocation *string
	flCloudRunPlatform        *string
)

const (
	appName         = `rundev-app`
	runRegion       = `us-central1`           // TODO(ahmetb) allow user to configure
	localRundevdURL = "http://localhost:8888" // TODO(ahmetb) allow user to configure (albeit, just for debugging/dev rundev itself, a.k.a -no-cloudrun)
	cleanupDeadline = time.Second * 1
)

func init() {
	log.SetFlags(log.Lmicroseconds)

	flLocalDir = flag.String("local-dir", ".", "local directory to sync")
	flRemoteDir = flag.String("remote-dir", "", "remote directory to sync (inside the container), defaults to container's WORKDIR")
	flAddr = flag.String("addr", "localhost:8080", "network address to start the local proxy server")
	flBuildCmd = flag.String("build-cmd", "", "(optional) command to re-build code (inside the container) after syncing,"+
		"inferred from Dockerfile by default (add comment on RUN directives like #rundev")
	flRunCmd = flag.String("run-cmd", "", "(optional) command to start application (inside the container) after syncing, inferred from Dockerfile by default")

	flNoCloudRun = flag.Bool("no-cloudrun", false, "do not deploy to Cloud Run (you should start rundevd on localhost:8888)")
	flCloudRunName = flag.String("name", appName, "name of the Cloud Run service")
	flCloudRunPlatform = flag.String("platform", "managed", "(passthrough to gcloud) managed or gke")
	flCloudRunCluster = flag.String("cluster", "", "(passthrough to gcloud) required when -platform=gke")
	flCloudRunClusterLocation = flag.String("cluster-location", "", "(passthrough to gcloud) required when -platform=gke")
	flag.Parse()
}

func main() {
	clientSecret := uuid.New().String()
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

	if *flCloudRunPlatform == "" {
		log.Fatal("-platform is empty")
	} else if *flCloudRunPlatform != cloudRunManagedPlatform {
		if *flCloudRunCluster == "" {
			log.Fatal("-cluster is empty, must be supplied when -platform is specified")
		} else if *flCloudRunClusterLocation == "" {
			log.Fatal("-cluster-location is empty, must be supplied when -platform is specified")
		}
	}
	var fileIgnores *ignore.FileIgnores
	var ignoreRules []string
	if f, err := os.Open(filepath.Join(*flLocalDir, ".dockerignore")); err == nil {
		defer f.Close()
		ignoreRules, err = ignore.ParseDockerignore(f)
		if err != nil {
			log.Fatalf("failed to parse .dockerignore: %+v", err)
		}
		fileIgnores = ignore.NewFileIgnores(ignoreRules)
		log.Printf("[info] parsed %d rules from .dockerignore file", len(ignoreRules))
	} else if os.IsNotExist(err) {
		log.Printf("if there are files you don't want to sync, you can create a .dockerignore file")
	} else {
		log.Fatalf("failed attempt to read .dockerignore file: %+v", err)
	}

	var rundevdURL string
	if *flNoCloudRun {
		rundevdURL = localRundevdURL
		log.Printf("not deploying to Cloud Run. make sure to start rundevd at %s", rundevdURL)
	} else {
		if *flCloudRunName == "" {
			log.Fatal("-name is empty")
		}
		log.Printf("starting one-time \"build & push & deploy\" to Cloud Run")
		project, err := currentProject(ctx)
		if err != nil {
			log.Fatalf("error reading current project ID from gcloud: %+v", err)
		}
		if project == "" {
			log.Fatalf("default project not set on gcloud. run: gcloud config set core/project PROJECT_NAME")
		}
		imageName := `gcr.io/` + project + `/` + *flCloudRunName

		df, err := readDockerfile(*flLocalDir)
		if err != nil {
			log.Fatal(err)
		}
		d, err := parseDockerfile(df)
		if err != nil {
			log.Fatalf("failed to parse Dockerfile: %+v", err)
		}
		var runCmd cmd
		if *flRunCmd == "" {
			runCmd, err = parseEntrypoint(d)
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

		var buildCmds types.BuildCmds
		if *flBuildCmd == "" {
			blCmds := parseBuildCmds(d)
			if len(blCmds) == 0 {
				log.Printf("[info] -build-cmd not specified: if you have steps to build your code after syncing, use this flag, or add #rundev comment to RUN statements in your Dockerfile")
			} else {
				log.Printf("[info] discovered build cmds (annotated with #rundev) from dockerfile as -build-cmd:")
				for _, v := range blCmds {
					log.Printf("-> %s", v)
					buildCmds = append(buildCmds, types.BuildCmd{
						C:  v.Flatten(),
						On: nil, // TODO(ahmetb) parse .Pattern
					})
				}
			}
		} else {
			argv, err := shlex.Split(*flBuildCmd)
			if err != nil {
				log.Fatalf("failed to parse -build-cmd into commands and args: %+v", err)
			}
			log.Printf("[info] parsed -build-cmd as: %s", argv)
			buildCmds = []types.BuildCmd{
				{
					C:  argv,
					On: nil,
				},
			}
		}

		ro := remoteRunOpts{
			syncDir:      *flRemoteDir,
			runCmd:       runCmd,
			buildCmds:    buildCmds,
			clientSecret: clientSecret,
			ignoreRules:  ignoreRules, // TODO(ahmetb) use this
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
		appURL, err := deployCloudRun(ctx, cloudrunOpts{
			platform:        *flCloudRunPlatform,
			project:         project,
			region:          runRegion,
			cluster:         *flCloudRunCluster,
			clusterLocation: *flCloudRunClusterLocation,
		}, *flCloudRunName, imageName)
		if err != nil {
			log.Fatalf("error deploying to Cloud Run: %+v", err)
		}
		defer cleanupCloudRun(*flCloudRunName, project, runRegion, cleanupDeadline)
		rundevdURL = appURL
	}
	sync := newSyncer(syncOpts{
		localDir:     *flLocalDir,
		targetAddr:   rundevdURL,
		clientSecret: clientSecret,
		ignores:      fileIgnores,
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
	log.Printf("local proxy server starting at http://%s (proxying to %s)", *flAddr, rundevdURL)
	if err := localServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("local server shut down gracefully, exiting")
		} else {
			log.Fatalf("local server failed to start: %+v", err)
		}
	}
}
