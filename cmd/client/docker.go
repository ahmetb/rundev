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
	"strings"

	"github.com/pkg/errors"
)

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

func parseDockerfileEntrypoint(b []byte) (cmd, error) {
	var c cmd
	r, err := parser.Parse(bytes.NewReader(b))
	if err != nil {
		return c, errors.Wrap(err, "error parsing dockerfile")
	}
	if r.AST == nil {
		return c, errors.Wrap(err, "ast was nil")
	}

	var epVals, cmdVals []string
	for _, stmt := range r.AST.Children {
		switch stmt.Value {
		case "from":
			c = cmd{} // reset (new stage)
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
	var buildCmdJSON, runCmdJSON string
	v, _ := json.Marshal(opts.runCmd)
	runCmdJSON = string(v)

	if len(opts.buildCmd) > 0 {
		v, _ = json.Marshal(opts.buildCmd)
		buildCmdJSON = string(v)
	}

	cmd := []string{"./rundevd",
		"-addr=:8080",
		"-run-cmd", runCmdJSON}
	if buildCmdJSON != "" {
		cmd = append(cmd, "-build-cmd", buildCmdJSON)
	}
	if opts.syncDir != "" {
		cmd = append(cmd, "-sync-dir="+opts.syncDir)
	}
	sw := new(strings.Builder)
	sw.WriteString("ENTRYPOINT [")
	for i, a := range cmd {
		fmt.Fprintf(sw, "%q", a)
		if i != len(cmd)-1 {
			sw.WriteString(`, `)
		}
	}
	sw.WriteString(`]`)
	sw.WriteString("\nCMD []")
	return sw.String()
}
