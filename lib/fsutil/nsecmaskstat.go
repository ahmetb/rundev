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
