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

import "path/filepath"

type DiffType int

const (
	DiffOpAdd DiffType = iota
	DiffOpDel
)

type DiffOp struct {
	Type DiffType
	Path string
}

func (o DiffOp) String() string {
	var s string
	switch o.Type {
	case DiffOpAdd:
		s = "A"
	case DiffOpDel:
		s = "D"
	default:
		s = "?"
	}
	return s + " " + o.Path
}

// FSDiff returns the operations that needs to be done on n2 to make it look like n1.
func FSDiff(n1, n2 FSNode) []DiffOp {
	return fsDiffInner(n1, n2, ".")
}

func fsDiffInner(n1, n2 FSNode, base string) []DiffOp {
	var ops []DiffOp
	ln := n1.Nodes
	rn := n2.Nodes
	for len(ln) > 0 && len(rn) > 0 {
		l, r := ln[0], rn[0]

		if l.Name < r.Name { // file doesn't exist in r
			ops = append(ops, DiffOp{Type: DiffOpAdd, Path: canonicalPath(base, l.Name)})
			ln = ln[1:] // advance
		} else if l.Name > r.Name { // file doesn't exist in l
			ops = append(ops, DiffOp{Type: DiffOpDel, Path: canonicalPath(base, r.Name)})
			rn = rn[1:]
		} else { // l.Name == r.Name (same item)
			if l.Mode.IsDir() != r.Mode.IsDir() { // one of them is a directory
				ops = append(ops, DiffOp{Type: DiffOpDel, Path: canonicalPath(base, l.Name)})
				ops = append(ops, DiffOp{Type: DiffOpAdd, Path: canonicalPath(base, l.Name)})
			} else if l.checksum() != r.checksum() {
				if !l.Mode.IsDir() && !r.Mode.IsDir() {
					// Nodes are not dir, re-upload file
					ops = append(ops, DiffOp{Type: DiffOpAdd, Path: canonicalPath(base, l.Name)})
				} else {
					// both Nodes are dir, recurse:
					ops = append(ops, fsDiffInner(l, r, canonicalPath(base, l.Name))...)
				}
			}
			ln, rn = ln[1:], rn[1:]
		}
	}
	// add remaining
	for _, l := range ln {
		ops = append(ops, DiffOp{Type: DiffOpAdd, Path: canonicalPath(base, l.Name)})
	}
	for _, r := range rn {
		ops = append(ops, DiffOp{Type: DiffOpDel, Path: canonicalPath(base, r.Name)})
	}
	return ops
}

// canonicalPath joins base and rel to create a canonical path string with unix path separator (/) independent of
// current platform.
func canonicalPath(base, rel string) string {
	return filepath.ToSlash(filepath.Join(base, rel))
}
