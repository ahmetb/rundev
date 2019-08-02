package main

import (
	"encoding/json"
	"flag"
	"github.com/ahmetb/rundev/lib/fsutil"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var (
	flOps string
	flDir string
)

func init() {
	flag.StringVar(&flOps, "ops-file", "", "json array file containing diff ops")
	flag.StringVar(&flDir, "dir", ".", "directory to look files for")
	flag.Parse()
}

func main() {
	log.SetOutput(os.Stderr)
	if flOps == "" {
		log.Fatal("-ops-file not specified")
	} else if flDir == "" {
		log.Fatal("-dir is empty")
	}
	var ops []fsutil.DiffOp
	b, err := ioutil.ReadFile(flOps)
	if err != nil {
		log.Fatalf("failed to open file: %+v", err)
	}
	if err := json.Unmarshal(b, &ops); err != nil {
		log.Fatalf("unmarshal error")
	}

	tar, err := fsutil.PatchArchive(flDir, ops)
	if err != nil {
		log.Fatalf("error creating patch archive: %+v", err)
	}
	io.Copy(os.Stdout, tar)
}
