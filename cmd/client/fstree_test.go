package main

import (
	"testing"
	"time"
)

func Test_fsNode_checksum(t *testing.T) {
	f := func() fsNode {
		return fsNode{
			name:  "n1",
			mode:  1,
			size:  2,
			mtime: time.Unix(1564448884, 0)}
	}

	n := f()

	m := f()
	if a, b := n.checksum(), m.checksum(); a != b {
		t.Fatal("checksums aren't idempotent")
	}

	m = f()
	m.name = "other"
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("name change didn't trigger checksum change")
	}

	m = f()
	m.size = 999
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("size change didn't trigger checksum change")
	}

	m = f()
	m.mode = 123
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("mode change didn't trigger checksum change")
	}

	m = f()
	m.mtime = time.Now()
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("mtime change didn't trigger checksum change")
	}

	m = f()
	m.nodes = []fsNode{f()}
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("nodes change didn't trigger checksum change")
	}

	n1, n2 := f(), f()
	n1.nodes = []fsNode{f(), f()}
	n2.nodes = []fsNode{f(), f()}
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("tree checksums are not idempotent")
	}
	n2.nodes[1].mode |= 0x1
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("different child node led to the same checksum")
	}
}
