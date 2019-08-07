package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
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
	runCmd          *cmd
	buildCmd        *cmd
	childPort       int
	portWaitTimeout time.Duration
}

type daemonServer struct {
	opts         daemonOpts
	reverseProxy http.Handler
	portCheck    portChecker

	procLogs  *bytes.Buffer
	patchLock sync.Mutex

	nannyLock sync.Mutex
	procNanny nanny
}

func newDaemonServer(opts daemonOpts) http.Handler {
	logs := new(bytes.Buffer)
	r := &daemonServer{
		opts:      opts,
		procLogs:  logs,
		portCheck: newTCPPortChecker(opts.childPort),
		procNanny: newProcessNanny(opts.runCmd.cmd, opts.runCmd.args, procOpts{
			port: opts.childPort,
			dir:  opts.syncDir,
			logs: logs,
		}),
	}
	r.reverseProxy = newReverseProxy(&url.URL{
		Scheme: "http",
		Host:   "localhost:" + strconv.Itoa(opts.childPort)})

	mux := http.NewServeMux()
	mux.HandleFunc("/rundevd/fsz", r.fsHandler)
	mux.HandleFunc("/rundevd/debugz", r.statusHandler)
	mux.HandleFunc("/rundevd/procz", r.logsHandler)
	mux.HandleFunc("/rundevd/restart", r.restart)
	mux.HandleFunc("/rundevd/patch", r.patch)
	mux.HandleFunc("/rundevd/", r.unsupported) // prevent proxying daemon debug endpoints to user app
	mux.HandleFunc("/", r.reverseProxyHandler)
	return mux
}

// newReverseProxy returns a reverse proxy to the user’s app.
func newReverseProxy(target *url.URL) http.Handler {
	return httputil.NewSingleHostReverseProxy(target)
}

func (srv *daemonServer) reverseProxyHandler(w http.ResponseWriter, req *http.Request) {
	log.Printf("[reverse proxy] path=%s method=%s", req.URL.Path, req.Method)

	reqChecksumHdr := req.Header.Get(constants.HdrRundevChecksum)
	if reqChecksumHdr == "" {
		writeErrorResp(w, http.StatusBadRequest, errors.Errorf("missing %s header from the client", constants.HdrRundevChecksum))
		return
	}
	reqChecksum, err := strconv.ParseUint(reqChecksumHdr, 10, 64)
	if reqChecksumHdr == "" {
		writeErrorResp(w, http.StatusBadRequest, errors.Wrapf(err, "malformed %s", constants.HdrRundevChecksum))
		return
	}

	fs, err := fsutil.Walk(srv.opts.syncDir)
	if err != nil {
		writeErrorResp(w, http.StatusInternalServerError, errors.Wrap(err, "failed to walk the sync directory"))
		return
	}
	respChecksum := fs.RootChecksum()
	w.Header().Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", respChecksum))

	if respChecksum != reqChecksum {
		writeChecksumMismatchResp(w, fs)
		return
	}
	srv.nannyLock.Lock()
	if !srv.procNanny.Running() {
		log.Printf("[reverse proxy] user process not running, restarting")
		if err := srv.procNanny.Restart(); err != nil {
			// TODO return structured response for errors
			writeProcError(w, fmt.Sprintf("failed to start child process: %+v", err), srv.procLogs.Bytes())
			srv.nannyLock.Unlock()
			return
		}
	}
	srv.nannyLock.Unlock()

	// wait for port to open
	ctx, cancel := context.WithTimeout(req.Context(), srv.opts.portWaitTimeout)
	defer cancel()
	if err := srv.portCheck.waitPort(ctx); err != nil {
		writeProcError(w, fmt.Sprintf("child process did not start listening on $PORT (%d) in %v", srv.opts.childPort, srv.opts.portWaitTimeout), srv.procLogs.Bytes())
		return
	}

	srv.reverseProxy.ServeHTTP(w, req)
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
	w.Header().Set(constants.HdrRundevChecksum, fmt.Sprintf("%v", fs.RootChecksum()))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fs); err != nil {
		log.Printf("ERROR: failed to encode json: %+v", err)
	}
}

func (srv *daemonServer) logsHandler(w http.ResponseWriter, req *http.Request) {
	srv.nannyLock.Lock()
	defer srv.nannyLock.Unlock()
	b := srv.procLogs.Bytes()
	w.Write(b)
}

func (srv *daemonServer) statusHandler(w http.ResponseWriter, req *http.Request) {
	fs, err := fsutil.Walk(srv.opts.syncDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	fmt.Fprintf(w, "fs checksum: %v\n", fs.RootChecksum())
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
	localChecksum := fmt.Sprintf("%d", fs.RootChecksum())
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

	if srv.opts.buildCmd != nil {
		log.Printf("rebuilding: %v", srv.opts.buildCmd)
		bc := exec.Command(srv.opts.buildCmd.cmd, srv.opts.buildCmd.args...)
		bc.Dir = srv.opts.syncDir
		if b, err := bc.CombinedOutput(); err != nil {
			writeProcError(w, fmt.Sprintf("rebuild command failed: %+v", err), b)
			return
		}
	}

	srv.nannyLock.Lock()
	if err := srv.procNanny.Restart(); err != nil {
		writeProcError(w, fmt.Sprintf("failed to restart subprocess after patching: %+v", err), srv.procLogs.Bytes())
	}
	srv.nannyLock.Unlock()

	w.WriteHeader(http.StatusAccepted)
	log.Printf("patch (%s) accepted", incomingChecksum)
	return
}

func writeProcError(w http.ResponseWriter, msg string, logs []byte) {
	w.Header().Set("Content-Type", constants.MimeProcessError)
	w.WriteHeader(http.StatusInternalServerError)
	resp := constants.ProcError{
		Message: msg,
		Output:  string(logs),
	}
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	if err := e.Encode(resp); err != nil {
		log.Printf("[WARNING] failed to encode process error into response body: %+v", err)
	}
}

func writeErrorResp(w http.ResponseWriter, code int, err error) {
	w.WriteHeader(code)
	fmt.Fprint(w, err.Error())
}

func writeChecksumMismatchResp(w http.ResponseWriter, fs fsutil.FSNode) {
	w.Header().Set(constants.HdrRundevChecksum, fmt.Sprintf("%d", fs.RootChecksum()))
	w.Header().Set("Content-Type", constants.MimeChecksumMismatch)
	w.WriteHeader(http.StatusPreconditionFailed)

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(fs); err != nil {
		log.Printf("WARNING: %+v", errors.Wrap(err, "error while marshaling remote fs"))
	}
	io.Copy(w, &b)
}
