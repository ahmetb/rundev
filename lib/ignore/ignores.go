package ignore

import (
	"github.com/bmatcuk/doublestar"
	"path/filepath"
	"strings"
)

type FileIgnores struct {
	patterns []string
}

// Ignored tests if given relative path is excluded. If FileIgnores is nil,
// it would not ignore any files.
func (f *FileIgnores) Ignored(path string) bool {
	if f == nil {
		return false
	}
	return ignored(path, f.patterns)
}

func NewFileIgnores(rules []string) *FileIgnores { return &FileIgnores{patterns: rules} }

// ignored checks if the path (OS-dependent file separator) matches one of the exclusion rules (that are in dockerignore format).
func ignored(path string, exclusions []string) bool {
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
