package main

import (
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/fsutil"
	"log"
	"net/http"
)

type cmd struct {
	cmd  string
	args []string
}

type daemonOpts struct {
	syncDir  string
	runCmd   cmd
	buildCmd cmd
}

type daemonServer struct {
	opts  daemonOpts
	child nanny
}

func newDaemonServer(opts daemonOpts) http.Handler {
	r := &daemonServer{
		opts:  opts,
		child: newProcessNanny(opts.runCmd.cmd, opts.runCmd.args)}

	mux := http.NewServeMux()
	mux.HandleFunc("/rundevd/fsz", r.fsHandler)
	mux.HandleFunc("/rundevd/restart", r.restart)
	return mux
}

func (srv *daemonServer) restart(w http.ResponseWriter, req *http.Request) {
	if err := srv.child.Restart(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error restarting process: %+v", err)
		return
	}
	fmt.Fprintf(w, "ok")
}

func (srv *daemonServer) fsHandler(w http.ResponseWriter, req *http.Request) {
	fs, err := fsutil.Walk(srv.opts.syncDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fs); err != nil {
		log.Printf("ERROR: failed to encode json: %+v", err)
	}
}
