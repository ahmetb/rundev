package fsutil

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_fsDiff_empty(t *testing.T) {
	expected := []DiffOp(nil)
	got := FSDiff(FSNode{Name: "root1"}, FSNode{Name: "root2"})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_rootDirNameNotCompared(t *testing.T) {
	fs1 := FSNode{
		Name:  "root1",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt"}}}
	fs2 := FSNode{
		Name:  "root2",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt"}}}

	got := FSDiff(fs1, fs2)
	expected := []DiffOp(nil)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_leftSideEmpty(t *testing.T) {
	fs1 := FSNode{
		Name: "root1",
		Mode: os.ModeDir | os.ModePerm}
	fs2 := FSNode{
		Name:  "root2",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt"}}}

	expected := []DiffOp{
		{DiffOpDel, "a.txt"},
		{DiffOpDel, "b.txt"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_rightSideEmpty(t *testing.T) {
	fs1 := FSNode{
		Name:  "root2",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt"}}}
	fs2 := FSNode{
		Name: "root1",
		Mode: os.ModeDir | os.ModePerm}

	expected := []DiffOp{
		{DiffOpAdd, "a.txt"},
		{DiffOpAdd, "b.txt"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_fileModification(t *testing.T) {
	fs1 := FSNode{
		Name:  "root1",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt", Mode: 0644}}}
	fs2 := FSNode{
		Name:  "root2",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt", Mode: 0600}}}

	expected := []DiffOp{
		{DiffOpAdd, "b.txt"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_fileAddDelete(t *testing.T) {
	fs1 := FSNode{
		Name:  "root1",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "a.txt"}, {Name: "b.txt"}}}
	fs2 := FSNode{
		Name:  "root2",
		Mode:  os.ModeDir | os.ModePerm,
		Nodes: []FSNode{{Name: "b.txt"}, {Name: "c.txt"}}}

	expected := []DiffOp{
		{DiffOpAdd, "a.txt"},
		{DiffOpDel, "c.txt"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_subDirectory(t *testing.T) {
	fs1 := FSNode{
		Name: "root1",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{Name: "a.txt"},
			{Name: "subdir",
				Mode:  os.ModeDir | os.ModePerm,
				Nodes: []FSNode{{Name: "b.txt"}, {Name: "c.txt"}},
			}}}
	fs2 := FSNode{
		Name: "root2",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{Name: "a.txt"},
		}}

	expected := []DiffOp{
		{DiffOpAdd, "subdir"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("expected add subdir, got diff:\n%s", diff)
	}

	expected = []DiffOp{
		{DiffOpDel, "subdir"},
	}
	got = FSDiff(fs2, fs1)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("expecte del subdir, got diff:\n%s", diff)
	}
}

func Test_fsDiff_directoryChangedToFile(t *testing.T) {
	fs1 := FSNode{
		Name: "root1",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{
				Name:  "subdir",
				Mode:  os.ModeDir | os.ModePerm,
				Nodes: []FSNode{{Name: "file1"}, {Name: "file2"}},
			},
		},
	}

	fs2 := FSNode{
		Name: "root2",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{
				Name: "subdir",
				Mode: 0644, // now  a file!
			},
		},
	}

	expected := []DiffOp{
		{DiffOpDel, "subdir"},
		{DiffOpAdd, "subdir"},
	}

	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}

	// switching the order shouldn't make any difference in diff ops
	got = FSDiff(fs2, fs1)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

func Test_fsDiff_interleaved(t *testing.T) {
	fs1 := FSNode{
		Name: "root1",

		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{
				Name:  "subdir",
				Mode:  os.ModeDir,
				Nodes: []FSNode{{Name: "a0"}, {Name: "a1"}, {Name: "a3"}, {Name: "a7"}},
			},
		},
	}
	fs2 := FSNode{
		Name: "root1",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{
				Name:  "subdir",
				Mode:  os.ModeDir,
				Nodes: []FSNode{{Name: "a1"}, {Name: "a2"}, {Name: "a4"}, {Name: "a5"}, {Name: "a6"}, {Name: "a8"}},
			},
		},
	}

	expected := []DiffOp{
		{DiffOpAdd, "subdir/a0"},
		{DiffOpDel, "subdir/a2"},
		{DiffOpAdd, "subdir/a3"},
		{DiffOpDel, "subdir/a4"},
		{DiffOpDel, "subdir/a5"},
		{DiffOpDel, "subdir/a6"},
		{DiffOpAdd, "subdir/a7"},
		{DiffOpDel, "subdir/a8"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}

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
	fs1 := FSNode{
		Name: "root1",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{Name: "e1",
				Mode: os.ModeDir | os.ModePerm,
				Nodes: []FSNode{
					{Name: "e1c1"},
					{Name: "e1c2"},
				}},
			{Name: "e3",
				Mode:  os.ModeDir | os.ModePerm,
				Nodes: []FSNode{{Name: "e3c1"}}},
			{Name: "e4",
				Mode:  os.ModeDir | os.ModePerm,
				Nodes: nil},
		},
	}

	fs2 := FSNode{
		Name: "root2",
		Mode: os.ModeDir | os.ModePerm,
		Nodes: []FSNode{
			{Name: "e1",
				Mode: os.ModeDir | os.ModePerm,
				Nodes: []FSNode{
					{Name: "e1c1", Size: 200},
					{Name: "e1c3"},
				}},
			{Name: "e2",
				Mode: os.ModeDir | os.ModePerm},
			{Name: "e4",
				Mode: os.ModeDir | os.ModePerm},
			{Name: "e5",
				Mode:  os.ModeDir | os.ModePerm,
				Nodes: []FSNode{{Name: "e5c1"}}},
		},
	}

	expected := []DiffOp{
		{DiffOpAdd, "e1/e1c1"},
		{DiffOpAdd, "e1/e1c2"},
		{DiffOpDel, "e1/e1c3"},
		{DiffOpDel, "e2"},
		{DiffOpAdd, "e3"},
		{DiffOpDel, "e5"},
	}
	got := FSDiff(fs1, fs2)
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatalf("diff:\n%s", diff)
	}
}
