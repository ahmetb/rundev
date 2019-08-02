package fsutil

import (
	"os"
	"time"
)

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
