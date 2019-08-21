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
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"google.golang.org/api/googleapi"
	run "google.golang.org/api/run/v1alpha1"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

const (
	cloudRunManagedPlatform = "managed"
)

type cloudrunOpts struct {
	platform string
	project  string

	region          string // managed only
	cluster         string // gke only
	clusterLocation string // gke only
}

func deployCloudRun(ctx context.Context, opts cloudrunOpts, appName, image string) (string, error) {
	args := []string{
		"--project=" + opts.project,
		"--platform=" + opts.platform,
	}
	deployArgs := []string{
		"--image=" + image,
	}
	if opts.platform == cloudRunManagedPlatform {
		args = append(args, "--region="+opts.region)
		deployArgs = append(deployArgs, "--allow-unauthenticated")
	} else {
		args = append(args, "--cluster="+opts.cluster)
		args = append(args, "--cluster-location="+opts.clusterLocation)
	}

	b, err := exec.CommandContext(ctx, "gcloud",
		append(append([]string{
			"alpha", "run", "deploy", "-q", appName}, args...), deployArgs...)...).CombinedOutput()
	if err != nil {
		return "", errors.Wrapf(err, "cloud run deployment failed. output:\n%s", string(b))
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "gcloud",
		append([]string{"beta", "run", "services", "describe", "-q", appName,
			"--format=get(status.url)"}, args...)...)
	cmd.Stderr = &stderr
	b, err = cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "cloud run describe failed. stderr:\n%s", string(stderr.Bytes()))
	}
	return strings.TrimSpace(string(b)), nil
}

// cleanupCloudRun fires and forgets a delete request to Cloud Run.
// TODO: make it work with CR-GKE as well.
// TODO: looks like we can shell out to gcloud (will add extra several secs) and handle CR-GKE too.
func cleanupCloudRun(appName, project, region string, timeout time.Duration) {
	log.Printf("cleaning up Cloud Run service %q", appName)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.TODO(), timeout)
	defer cleanupCancel()
	rs, err := run.NewService(cleanupCtx)
	if err != nil {
		log.Printf("[warn] failed to initialize cloudrun client: %+v", err)
		return
	}
	rs.BasePath = strings.Replace(rs.BasePath, "://", "://"+region+"-", 1)
	uri := fmt.Sprintf("namespaces/%s/services/%s", project, appName)
	_, err = rs.Namespaces.Services.Delete(uri).Do()
	if err == nil {
		log.Printf("cleanup successful")
		return
	}
	if v, ok := err.(*googleapi.Error); ok {
		if v.Code == http.StatusNotFound {
			log.Printf("cloud run app already seems to be gone, that's weird...")
			return
		}
		log.Printf("[warn] run api cleanup call responded with error: %+v\nbody: %s",
			v, v.Body)
	} else {
		log.Printf("[warn] calling run api for cleanup failed: %+v", err)
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
