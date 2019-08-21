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
