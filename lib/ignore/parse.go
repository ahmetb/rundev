package ignore

import (
	"github.com/bmatcuk/doublestar"
	"github.com/docker/docker/builder/dockerignore"
	"github.com/pkg/errors"
	"io"
	"path/filepath"
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

// Ignored checks if the path (OS-dependent file separator) matches one of the exclusion rules (that are in dockerignore format).
func Ignored(path string, exclusions []string) bool {
	unixPath := filepath.ToSlash(path)
	for _, p := range exclusions {
		ok, _ := pathMatch(unixPath, p) // ignore error as it's checked as part of parsing the pattern
		if ok {
			return true
		}
	}
	return false
}

// pathMatch checks if given path with forward slashes matches the dockerignore pattern.
// This supports double star (**) globbing, and patterns with leading slash (/).
func pathMatch(unixPath string, pattern string) (bool, error) {
	pattern = strings.TrimPrefix(pattern, "/") // .dockerignore supports /a/b format, but we just need to make it relative.
	return doublestar.Match(pattern, unixPath)
}
