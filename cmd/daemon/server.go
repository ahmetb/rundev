package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type cmd struct {
	cmd  string
	args []string
}

type daemonOpts struct {
	syncDir         string
	runCmd          cmd
	buildCmd        cmd
	childPort       int
	portWaitTimeout time.Duration
}

type daemonServer struct {
	opts daemonOpts

	patchLock sync.Mutex

	portCheck portChecker
	nannyLock sync.Mutex
	procNanny nanny
}

func newDaemonServer(opts daemonOpts) http.Handler {
	r := &daemonServer{
		opts:      opts,
		portCheck: newTCPPortChecker(opts.childPort),
		procNanny: newProcessNanny(opts.runCmd.cmd, opts.runCmd.args, procOpts{
			port: opts.childPort,
			dir:  opts.syncDir,
		}),
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
	mux.HandleFunc("/", r.ensureChildProcessHandler(rp))
	return mux
}

func (srv *daemonServer) ensureChildProcessHandler(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Printf("[reverse proxy] path=%s method=%s", req.URL.Path, req.Method)

		srv.nannyLock.Lock()
		if !srv.procNanny.Running() {
			log.Printf("[reverse proxy] user process not running, restarting")
			if err := srv.procNanny.Restart(); err != nil {
				// TODO return structured response for errors
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "failed to start child process. output:\n%v", err) // actually get output
				srv.nannyLock.Unlock()
				return
			}
		}
		srv.nannyLock.Unlock()

		// wait for port to open
		ctx, cancel := context.WithTimeout(req.Context(), srv.opts.portWaitTimeout)
		defer cancel()
		if err := srv.portCheck.waitPort(ctx); err != nil {
			// TODO return structured response for errors
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "child process did not start listening on $PORT in %v", srv.opts.portWaitTimeout)
			return
		}

		next.ServeHTTP(w, req)
	}
}

func (srv *daemonServer) restart(w http.ResponseWriter, req *http.Request) {
	if err := srv.procNanny.Restart(); err != nil {
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
	fmt.Fprintf(w, "child process running: %v\n", srv.procNanny.Running())
	fmt.Fprintf(w, "opts: %#v\n", srv.opts)
}

func (*daemonServer) unsupported(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "unsupported rundev daemon endpoint %s", req.URL.Path)
}

func (srv *daemonServer) patch(w http.ResponseWriter, req *http.Request) {
	srv.patchLock.Lock()
	defer srv.patchLock.Unlock()
	// TODO would be good to stop accepting new requests during this period

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

	defer req.Body.Close()
	if err := fsutil.ApplyPatch(srv.opts.syncDir, req.Body); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to uncompress patch tar: %+v", err)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	log.Printf("patch (%s) accepted", incomingChecksum)
	srv.nannyLock.Lock()
	srv.procNanny.Restart()
	srv.nannyLock.Unlock()
	return
}
