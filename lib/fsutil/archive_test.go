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

package fsutil

import (
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_expandDirEntries(t *testing.T) {
	tmp, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	files := []string{
		"empty/",
		"foo/",
		"foo/file1",
		"foo/file2",
		"foo/nested/",
		"foo/nested/1",
		"foo/nested/2",
		"foo/nested/empty/",
		"zoo1",
		"zoo2",
	}
	for _, f := range files {
		fp := filepath.Join(tmp, filepath.FromSlash(f))
		if strings.HasSuffix(f, "/") {
			if err := os.MkdirAll(strings.TrimRight(fp, "/"), 0755); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := ioutil.WriteFile(fp, []byte{}, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	expected := []string{
		filepath.Join(tmp, ""), // root entry
		filepath.Join(tmp, "empty"),
		filepath.Join(tmp, "foo"),
		filepath.Join(tmp, "foo/file1"),
		filepath.Join(tmp, "foo/file2"),
		filepath.Join(tmp, "foo/nested"),
		filepath.Join(tmp, "foo/nested/1"),
		filepath.Join(tmp, "foo/nested/2"),
		filepath.Join(tmp, "foo/nested/empty"),
		filepath.Join(tmp, "zoo1"),
		filepath.Join(tmp, "zoo2")}

	v, err := expandDirEntries(tmp)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, vv := range v {
		got = append(got, vv.fullPath)
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatal(diff)
	}
}
