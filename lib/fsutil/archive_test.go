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
