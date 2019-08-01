package fsutil

import (
	"testing"
	"time"
)

func Test_fsNode_checksum(t *testing.T) {
	f := func() FSNode {
		return FSNode{
			Name:  "n1",
			Mode:  1,
			Size:  2,
			Mtime: time.Unix(1564448884, 0)}
	}

	n := f()

	m := f()
	if a, b := n.Checksum(), m.Checksum(); a != b {
		t.Fatal("checksums aren't idempotent")
	}

	m = f()
	m.Name = "other"
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("Name change didn't trigger Checksum change")
	}

	m = f()
	m.Size = 999
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("Size change didn't trigger Checksum change")
	}

	m = f()
	m.Mode = 123
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("Mode change didn't trigger Checksum change")
	}

	m = f()
	m.Mtime = time.Now()
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("Mtime change didn't trigger Checksum change")
	}

	m = f()
	m.Nodes = []FSNode{f()}
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("Nodes change didn't trigger Checksum change")
	}

	n1, n2 := f(), f()
	n1.Nodes = []FSNode{f(), f()}
	n2.Nodes = []FSNode{f(), f()}
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("tree checksums are not idempotent")
	}
	n2.Nodes[1].Mode |= 0x1
	if a, b := n.Checksum(), m.Checksum(); a == b {
		t.Fatal("different child node led to the same Checksum")
	}
}
