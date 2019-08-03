package main

import (
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

type cmd struct {
	cmd  string
	args []string
}

type daemonOpts struct {
	syncDir   string
	runCmd    cmd
	buildCmd  cmd
	childPort int
}

type daemonServer struct {
	opts daemonOpts

	patchLock sync.Mutex
	child     nanny
}

func newDaemonServer(opts daemonOpts) http.Handler {
	r := &daemonServer{
		opts: opts,
		child: newProcessNanny(opts.runCmd.cmd, opts.runCmd.args, procOpts{
			port: opts.childPort}),
	}
	rp := newReverseProxy(
		&url.URL{
			Scheme: "http",
			Host:   "localhost:" + strconv.Itoa(opts.childPort)},
		syncOpts{
			syncDir: opts.syncDir})

	mux := http.NewServeMux()
	mux.HandleFunc("/rundevd/fsz", r.fsHandler)
	mux.HandleFunc("/rundevd/debugz", r.debugHandler)
	mux.HandleFunc("/rundevd/restart", r.restart)
	mux.HandleFunc("/rundevd/patch", r.patch)
	mux.HandleFunc("/rundevd/", r.unsupported) // prevent proxying daemon debug endpoints to user app
	// TODO(ahmetb) add /rundevd/upload
	mux.Handle("/", rp)
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
	w.Header().Set(constants.HdrRundevChecksum, fmt.Sprintf("%v", fs.Checksum()))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fs); err != nil {
		log.Printf("ERROR: failed to encode json: %+v", err)
	}
}

func (srv *daemonServer) debugHandler(w http.ResponseWriter, req *http.Request) {
	fs, err := fsutil.Walk(srv.opts.syncDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	fmt.Fprintf(w, "fs checksum: %v\n", fs.Checksum())
	fmt.Fprintf(w, "child process running: %v\n", srv.child.Running())
	fmt.Fprintf(w, "opts: %#v\n", srv.opts)
}

func (*daemonServer) unsupported(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "unsupported rundev daemon endpoint %s", req.URL.Path)
}

func (srv *daemonServer) patch(w http.ResponseWriter, req *http.Request) {
	srv.patchLock.Lock()
	defer srv.patchLock.Unlock()

	if req.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if ct := req.Header.Get("content-type"); ct != constants.MimePatch {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	incomingChecksum := req.Header.Get(constants.HdrRundevChecksum)
	if incomingChecksum == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "patch request did not contain %s header", constants.HdrRundevChecksum)
		return
	}
	expectedLocalChecksum := req.Header.Get(constants.HdrRundevPatchPreconditionSum)
	if expectedLocalChecksum == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "patch request did not contain %s header", constants.HdrRundevPatchPreconditionSum)
		return
	}

	fs, err := fsutil.Walk(srv.opts.syncDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	localChecksum := fmt.Sprintf("%d", fs.Checksum())
	if localChecksum == incomingChecksum {
		// no-op, already in sync
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if localChecksum != expectedLocalChecksum {
		w.WriteHeader(http.StatusPreconditionFailed)
		w.Header().Set(constants.HdrRundevChecksum, localChecksum)
		return
	}
	// would be good to stop serving all requests during this period also

	defer req.Body.Close()
	if err := fsutil.ApplyPatch(srv.opts.syncDir, req.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to uncompress patch tar: %+v", err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	return
}
