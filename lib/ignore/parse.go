package ignore

import (
	"github.com/docker/docker/builder/dockerignore"
	"github.com/pkg/errors"
	"io"
	"strings"
)

// ParseDockerignore returns statements in a .dockerignore file contents
// that can be matched to files with filepath.Match.
// https://docs.docker.com/engine/reference/builder/#dockerignore-file
func ParseDockerignore(r io.Reader) ([]string, error) {
	v, err := dockerignore.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse dockerignore format")
	}

	// TODO: it looks like implementing exceptions (!PATTERN) will be difficult for now. they're also rarely used.
	for _, p := range v {
		if strings.HasPrefix(p, "!") {
			return nil, errors.Errorf("exception rules in dockerignores are not yet supported (pattern: %s)", p)
		}
	}

	// validate paths
	for _, p := range v {
		if _, err := pathMatch(".", p); err != nil {
			return nil, errors.Wrapf(err, "failed to parse dockerignore pattern %s", p)
		}
	}
	return v, nil
}
