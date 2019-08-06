package main

import (
	"bytes"
	"context"
	"fmt"
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
	cmd := []string{"./rundevd",
		"-addr=:8080",
		"-run-cmd", opts.runCmd}
	if opts.buildCmd != "" {
		cmd = append(cmd, "-build-cmd", opts.buildCmd)
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
	return sw.String()
}
