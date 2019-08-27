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
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

const (
	dumbInitURL = `https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64`
	rundevdURL = `https://storage.googleapis.com/rundev-test/rundevd-v0.0.0-919089f`
)

var (
	runCmdAnnotationPattern = regexp.MustCompile(`#\s?rundev$`)
)

type remoteRunOpts struct {
	syncDir      string
	runCmd       cmd
	buildCmds    []cmd
	clientSecret string
	ignoreRules  []string
}

type buildOpts struct {
	dir        string
	image      string
	dockerfile []byte
}

type cmd struct {
	cmd  string
	args []string
}

func (c cmd) String() string {
	s := c.cmd
	for _, a := range c.args {
		s += fmt.Sprintf(" %q", a)
	}
	return s
}

func (c cmd) List() []string {
	if c.cmd == "" {
		return nil
	}
	return append([]string{c.cmd}, c.args...)
}

type dockerfile struct {
	syntaxTree *parser.Node
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

func parseDockerfile(b []byte) (*dockerfile, error) {
	r, err := parser.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, errors.Wrap(err, "error parsing dockerfile")
	}
	if r.AST == nil {
		return nil, errors.Wrap(err, "ast was nil")
	}
	return &dockerfile{r.AST}, nil
}

func parseEntrypoint(d *dockerfile) (cmd, error) {
	var c cmd
	var epVals, cmdVals []string
	for _, stmt := range d.syntaxTree.Children {
		switch stmt.Value {
		case "from":
			// reset (new stage)
			epVals = nil
			cmdVals = nil
		case "entrypoint":
			epVals = parseCommand(stmt.Next, stmt.Attributes["json"])
		case "cmd":
			cmdVals = parseCommand(stmt.Next, stmt.Attributes["json"])
		}
	}
	if len(epVals) == 0 && len(cmdVals) == 0 {
		return c, errors.New("no CMD or ENTRYPOINT values in dockerfile")
	}
	if len(epVals) == 0 {
		// CMD becomes the entrypoint
		return cmd{cmdVals[0], cmdVals[1:]}, nil
	}
	// merge ENTRYPOINT argv and CMD values
	return cmd{epVals[0], append(epVals[1:], cmdVals...)}, nil
}

// parseBuildCmds extracts RUN commands from the last dockerfile stage with #rundev annotation.
func parseBuildCmds(d *dockerfile) []cmd {
	var out []cmd
	for _, stmt := range d.syntaxTree.Children {
		switch stmt.Value {
		case "from":
			out = nil // reset
		case "run":
			if runCmdAnnotationPattern.MatchString(stmt.Original) {
				c := parseCommand(stmt.Next, stmt.Attributes["json"])
				cm := cmd{c[0], c[1:]}
				// trim comment at the end of argv (as dockerfile parser isn't doing so)
				if len(cm.args) > 0 {
					v := cm.args[len(cm.args)-1]
					v = runCmdAnnotationPattern.ReplaceAllString(v, "")
					v = strings.TrimRightFunc(v, unicode.IsSpace)
					cm.args[len(cm.args)-1] = v
				}
				out = append(out, cm)
			}
		}
	}
	return out
}

// parseCommand parses CMD and ENTRYPOINT nodes, based on whether they're JSON lists or not.
// Non-JSON values are wrapped in [/bin/sh -c VALUE]
func parseCommand(n *parser.Node, json bool) []string {
	var out []string
	for n != nil {
		if !json {
			return []string{"/bin/sh", "-c", n.Value}
		}
		out = append(out, n.Value)
		n = n.Next
	}
	return out
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
	rc, _ := json.Marshal(opts.runCmd.List())
	cmd := []string{"/bin/rundevd",
		"-client-secret=" + opts.clientSecret,
		"-run-cmd", string(rc)}

	if len(opts.buildCmds) > 0 {
		bcs := make([][]string, len(opts.buildCmds))
		for i, v := range opts.buildCmds {
			bcs[i] = v.List()
		}
		bc, _ := json.Marshal(bcs)
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
	fmt.Fprintln(sw,`ENTRYPOINT ["/bin/dumb_init", "--"]`)
	fmt.Fprintf(sw,`CMD [`)
	for i, a := range cmd {
		fmt.Fprintf(sw, "%q", a)
		if i != len(cmd)-1 {
			sw.WriteString(", \\\n\t")
		}
	}
	sw.WriteString(`]`)
	return sw.String()
}
