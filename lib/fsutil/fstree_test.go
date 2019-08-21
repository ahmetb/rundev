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
	"testing"
	"time"
)

func TestWalk(t *testing.T) {
	_, err := Walk("../..", nil) // TODO remove
	if err != nil {
		t.Fatal(err)
	}
}

func Test_checksum(t *testing.T) {
	f := func() FSNode {
		return FSNode{
			Name:  "n1",
			Mode:  1,
			Size:  2,
			Mtime: time.Unix(1564448884, 0)}
	}

	n := f()

	m := f()
	if a, b := n.checksum(), m.checksum(); a != b {
		t.Fatal("checksums aren't idempotent")
	}

	m = f()
	m.Name = "other"
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("Name change didn't trigger Checksum change")
	}

	m = f()
	m.Size = 999
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("Size change didn't trigger Checksum change")
	}

	m = f()
	m.Mode = 123
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("Mode change didn't trigger Checksum change")
	}

	m = f()
	m.Mtime = time.Now()
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("Mtime change didn't trigger Checksum change")
	}

	m = f()
	m.Nodes = []FSNode{f()}
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("Nodes change didn't trigger Checksum change")
	}

	n1, n2 := f(), f()
	n1.Nodes = []FSNode{f(), f()}
	n2.Nodes = []FSNode{f(), f()}
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("tree checksums are not idempotent")
	}
	n2.Nodes[1].Mode |= 0x1
	if a, b := n.checksum(), m.checksum(); a == b {
		t.Fatal("different child node led to the same Checksum")
	}
}

func TestChecksumRoot(t *testing.T) {
	fs := FSNode{
		Name:  "name1",
		Mode:  1,
		Size:  1,
		Mtime: time.Unix(1, 0),
		Nodes: nil,
	}

	c := fs.RootChecksum()
	fs.Name = "name2"

	if fs.RootChecksum() != c {
		t.Fatal("name change was not supposed to trigger root checksum change")
	}

	fs.Mode = 2
	if fs.RootChecksum() != c {
		t.Fatal("mode change was not supposed to trigger root checksum change")
	}

	fs.Size = 2
	if fs.RootChecksum() != c {
		t.Fatal("size change was not supposed to trigger root checksum change")
	}

	fs.Nodes = append(fs.Nodes, FSNode{Name: "bar"})
	if fs.RootChecksum() == c {
		t.Fatal("nodes was supposed to trigger root checksum change")
	}
}
