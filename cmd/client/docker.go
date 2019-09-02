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
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/types"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	dumbInitURL = `https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64`
	rundevdURL  = `https://storage.googleapis.com/rundev-test/rundevd-v0.0.0-919089f`
)

type remoteRunOpts struct {
	syncDir      string
	runCmd       types.Cmd
	buildCmds    types.BuildCmds
	clientSecret string
	ignoreRules  []string
}

type buildOpts struct {
	dir        string
	image      string
	dockerfile []byte
}

func dockerBuildPush(ctx context.Context, opts buildOpts) error {
	b, err := exec.CommandContext(ctx, "docker", "version").CombinedOutput()
	if err != nil {
		errors.Wrapf(err, "local docker engine is unreachable, output=%s", string(b))
	}
	args := []string{"build", "--tag=" + opts.image, opts.dir}
	if len(opts.dockerfile) > 0 {
		args = append(args, "--file=-")
	}
	cmd := exec.CommandContext(ctx,
		"docker", args...)
	if len(opts.dockerfile) > 0 {
		cmd.Stdin = bytes.NewReader(opts.dockerfile)
	}
	b, err = cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "building docker image failed, output=%s", string(b))
	}
	b, err = exec.CommandContext(ctx, "docker", "push", opts.image).CombinedOutput()
	return errors.Wrapf(err, "building docker image failed, output=%s", string(b))
}

func readDockerfile(dir string) ([]byte, error) {
	df, err := ioutil.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Errorf("Dockerfile not found at directory %s", dir)
		}
		return nil, errors.Wrap(err, "error reading Dockerfile")
	}
	return df, nil
}

func prepEntrypoint(opts remoteRunOpts) string {
	rc, _ := json.Marshal(opts.runCmd)
	cmd := []string{"/bin/rundevd",
		"-client-secret=" + opts.clientSecret,
		"-run-cmd", string(rc)}

	if len(opts.buildCmds) > 0 {
		bc, _ := json.Marshal(opts.buildCmds)
		cmd = append(cmd, "-build-cmds", string(bc))
	}
	if len(opts.ignoreRules) > 0 {
		b, _ := json.Marshal(opts.ignoreRules)
		cmd = append(cmd, "-ignore-patterns", string(b))
	}
	if opts.syncDir != "" {
		cmd = append(cmd, "-sync-dir="+opts.syncDir)
	}
	sw := new(strings.Builder)
	fmt.Fprintf(sw, "ADD %s /bin/dumb_init\n", dumbInitURL)
	fmt.Fprintf(sw, "ADD %s /bin/rundevd\n", rundevdURL)
	fmt.Fprintln(sw, "RUN chmod +x /bin/rundevd /bin/dumb_init")
	fmt.Fprintln(sw, `ENTRYPOINT ["/bin/dumb_init", "--"]`)
	fmt.Fprintf(sw, `CMD [`)
	for i, a := range cmd {
		fmt.Fprintf(sw, "%q", a)
		if i != len(cmd)-1 {
			sw.WriteString(", \\\n\t")
		}
	}
	sw.WriteString(`]`)
	return sw.String()
}
