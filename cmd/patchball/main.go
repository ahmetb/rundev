package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/ahmetb/rundev/lib/ignore"
	"io"
	"io/ioutil"
	"log"
	"os"
)

var (
	flOps          string
	flDir          string
	flDockerignore string
)

func init() {
	flag.StringVar(&flOps, "ops-file", "", "json array file containing diff ops")
	flag.StringVar(&flDir, "dir", ".", "directory to look files for")
	flag.StringVar(&flDockerignore, "dockerignore", "", "specify path to parse dockerignore rules")
	flag.Parse()
}

func main() {
	log.SetOutput(os.Stderr)
	if flOps == "" {
		log.Fatal("-ops-file not specified")
	} else if flDir == "" {
		log.Fatal("-dir is empty")
	}

	var ignores *ignore.FileIgnores
	if flDockerignore != "" {
		f, err := os.Open(flDockerignore)
		if err != nil {
			log.Fatalf("failed to open -dockerignore: %+v", err)
		}
		defer f.Close()
		r, err := ignore.ParseDockerignore(f)
		if err != nil {
			log.Fatalf("failed to parse -dockerignore: %+v", err)
		}
		ignores = ignore.NewFileIgnores(r)
		log.Printf("info: parsed %d ignore rules", len(r))
	}

	var ops []fsutil.DiffOp
	for _, op := range ops {
		fmt.Fprintf(os.Stderr, "%v\n", op)
	}
	b, err := ioutil.ReadFile(flOps)
	if err != nil {
		log.Fatalf("failed to open file: %+v", err)
	}
	if err := json.Unmarshal(b, &ops); err != nil {
		log.Fatalf("unmarshal error")
	}

	tar, _, err := fsutil.PatchArchive(flDir, ops, ignores)
	if err != nil {
		log.Fatalf("error creating patch archive: %+v", err)
	}
	io.Copy(os.Stdout, tar)
}
