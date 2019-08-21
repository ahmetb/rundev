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
	"os"
	"time"
)

// nanosecMaskingStat returns the nanosec portion of ModTime() in the underlying os.FileInfo.
type nanosecMaskingStat struct{ s os.FileInfo }

func (n nanosecMaskingStat) Name() string       { return n.s.Name() }
func (n nanosecMaskingStat) Size() int64        { return n.s.Size() }
func (n nanosecMaskingStat) Mode() os.FileMode  { return n.s.Mode() }
func (n nanosecMaskingStat) ModTime() time.Time { return n.s.ModTime().Truncate(time.Second) }
func (n nanosecMaskingStat) IsDir() bool        { return n.s.IsDir() }
func (n nanosecMaskingStat) Sys() interface{}   { return n.s.Sys() }

// zeroSizeStat returns zero size for the given os.FileInfo.
type zeroSizeStat struct{ s os.FileInfo }

func (c zeroSizeStat) Name() string       { return c.s.Name() }
func (c zeroSizeStat) Size() int64        { return 0 }
func (c zeroSizeStat) Mode() os.FileMode  { return c.s.Mode() }
func (c zeroSizeStat) ModTime() time.Time { return c.s.ModTime() }
func (c zeroSizeStat) IsDir() bool        { return c.s.IsDir() }
func (c zeroSizeStat) Sys() interface{}   { return c.s.Sys() }

type whiteoutStat struct {
	name string
	sys  interface{}
}

func (w whiteoutStat) Name() string     { return w.name }
func (whiteoutStat) Size() int64        { return 0 }
func (whiteoutStat) Mode() os.FileMode  { return 0444 }
func (whiteoutStat) ModTime() time.Time { return time.Unix(0, 0) }
func (whiteoutStat) IsDir() bool        { return false }
func (w whiteoutStat) Sys() interface{} { return w.sys }
