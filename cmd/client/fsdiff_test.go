package main

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_fsDiff_empty(t *testing.T) {
	expected := []diffOp(nil)
	got := fsDiff(fsNode{name: "root1"}, fsNode{name: "root2"})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_rootDirNameNotCompared(t *testing.T) {
	fs1 := fsNode{
		name:  "root1",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt"}}}
	fs2 := fsNode{
		name:  "root2",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt"}}}

	got := fsDiff(fs1, fs2)
	expected := []diffOp(nil)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_leftSideEmpty(t *testing.T) {
	fs1 := fsNode{
		name: "root1",
		mode: os.ModeDir | os.ModePerm}
	fs2 := fsNode{
		name:  "root2",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt"}}}

	expected := []diffOp{
		{diffOpDel, "a.txt"},
		{diffOpDel, "b.txt"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_rightSideEmpty(t *testing.T) {
	fs1 := fsNode{
		name:  "root2",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt"}}}
	fs2 := fsNode{
		name: "root1",
		mode: os.ModeDir | os.ModePerm}

	expected := []diffOp{
		{diffOpAdd, "a.txt"},
		{diffOpAdd, "b.txt"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_fileModification(t *testing.T) {
	fs1 := fsNode{
		name:  "root1",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt", mode: 0644}}}
	fs2 := fsNode{
		name:  "root2",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt", mode: 0600}}}

	expected := []diffOp{
		{diffOpAdd, "b.txt"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_fileAddDelete(t *testing.T) {
	fs1 := fsNode{
		name:  "root1",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a.txt"}, {name: "b.txt"}}}
	fs2 := fsNode{
		name:  "root2",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "b.txt"}, {name: "c.txt"}}}

	expected := []diffOp{
		{diffOpAdd, "a.txt"},
		{diffOpDel, "c.txt"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_subDirectory(t *testing.T) {
	fs1 := fsNode{
		name: "root1",
		mode: os.ModeDir | os.ModePerm,
		nodes: []fsNode{
			{name: "a.txt"},
			{name: "subdir",
				mode:  os.ModeDir | os.ModePerm,
				nodes: []fsNode{{name: "b.txt"}, {name: "c.txt"}},
			}}}
	fs2 := fsNode{
		name: "root2",
		mode: os.ModeDir | os.ModePerm,
		nodes: []fsNode{
			{name: "a.txt"},
		}}

	expected := []diffOp{
		{diffOpAdd, "subdir"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("expected add subdir, got diff:\n%s", diff)
	}

	expected = []diffOp{
		{diffOpDel, "subdir"},
	}
	got = fsDiff(fs2, fs1)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("expecte del subdir, got diff:\n%s", diff)
	}
}

func Test_fsDiff_directoryChangedToFile(t *testing.T) {
	fs1 := fsNode{
		name: "root1",
		mode: os.ModeDir | os.ModePerm,
		nodes: []fsNode{
			{
				name:  "subdir",
				mode:  os.ModeDir | os.ModePerm,
				nodes: []fsNode{{name: "file1"}, {name: "file2"}},
			},
		},
	}

	fs2 := fsNode{
		name: "root2",
		mode: os.ModeDir | os.ModePerm,
		nodes: []fsNode{
			{
				name: "subdir",
				mode: 0644, // now  a file!
			},
		},
	}

	expected := []diffOp{
		{diffOpDel, "subdir"},
		{diffOpAdd, "subdir"},
	}

	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_interleaved(t *testing.T) {
	fs1 := fsNode{
		name:  "root1",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a0"}, {name: "a1"}, {name: "a3"}, {name: "a7"}}}
	fs2 := fsNode{
		name:  "root1",
		mode:  os.ModeDir | os.ModePerm,
		nodes: []fsNode{{name: "a1"}, {name: "a2"}, {name: "a4"}, {name: "a5"}, {name: "a6"}, {name: "a8"}}}

	expected := []diffOp{
		{diffOpAdd, "a0"},
		{diffOpDel, "a2"},
		{diffOpAdd, "a3"},
		{diffOpDel, "a4"},
		{diffOpDel, "a5"},
		{diffOpDel, "a6"},
		{diffOpAdd, "a7"},
		{diffOpDel, "a8"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

// TODO(ahmetb) implement
func Test_fsDiff(t *testing.T) {
	// (fs1)			(fs2)
	//
	// .				.
	// ├-- e1           ├-- e1
	// |   ├-- e1c1     |   ├-- e1c1'
	// |   └-- e1c2     |   └–– e1c3
	// ├-- e3           ├-- e2
	// |   └-- e3c1     ├-- e4
	// └-- e4           └-- e5
	//                      └–– e5c1
	fs1 := fsNode{
		name: "root1",
		mode: os.ModeDir | os.ModePerm,
		nodes: []fsNode{
			{name: "e1",
				mode: os.ModeDir | os.ModePerm,
				nodes: []fsNode{
					{name: "e1c1"},
					{name: "e1c2"},
				}},
			{name: "e3",
				mode:  os.ModeDir | os.ModePerm,
				nodes: []fsNode{{name: "e3c1"}}},
			{name: "e4",
				mode:  os.ModeDir | os.ModePerm,
				nodes: nil},
		},
	}

	fs2 := fsNode{
		name: "root2",
		mode: os.ModeDir | os.ModePerm,
		nodes: []fsNode{
			{name: "e1",
				mode: os.ModeDir | os.ModePerm,
				nodes: []fsNode{
					{name: "e1c1", size: 200},
					{name: "e1c3"},
				}},
			{name: "e2",
				mode: os.ModeDir | os.ModePerm},
			{name: "e4",
				mode: os.ModeDir | os.ModePerm},
			{name: "e5",
				mode:  os.ModeDir | os.ModePerm,
				nodes: []fsNode{{name: "e5c1"}}},
		},
	}

	expected := []diffOp{
		{diffOpAdd, "e1/e1c1"},
		{diffOpAdd, "e1/e1c2"},
		{diffOpDel, "e1/e1c3"},
		{diffOpDel, "e2"},
		{diffOpAdd, "e3"},
		{diffOpDel, "e5"},
	}
	got := fsDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}
