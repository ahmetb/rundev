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
