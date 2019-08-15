package main

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/ahmetb/rundev/lib/handlerutil"
	"github.com/ahmetb/rundev/lib/ignore"
	"github.com/google/uuid"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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
	clientSecret    string
	syncDir         string
	runCmd          *cmd
	buildCmds       []cmd
	childPort       int
	ignores         *ignore.FileIgnores
	portWaitTimeout time.Duration
}

type daemonServer struct {
	opts         daemonOpts
	reverseProxy http.Handler
	portCheck    portChecker

	procLogs  *bytes.Buffer
	patchLock sync.RWMutex

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
	mux.HandleFunc("/rundevd/fsz", handlerutil.NewFSDebugHandler(r.opts.syncDir, r.opts.ignores))
	mux.HandleFunc("/rundevd/debugz", r.statusHandler)
	mux.HandleFunc("/rundevd/procz", r.logsHandler)
	mux.HandleFunc("/rundevd/restart", r.restartHandler)
	mux.HandleFunc("/rundevd/kill", r.killHandler)
	mux.HandleFunc("/rundevd/patch", withClientSecretAuth(opts.clientSecret, r.patch))
	mux.HandleFunc("/rundevd/", handlerutil.NewUnsupportedDebugEndpointHandler())
	mux.HandleFunc("/", r.reverseProxyHandler)
	return mux
}

func withClientSecretAuth(secret string, hand http.HandlerFunc) http.HandlerFunc {
	if secret == "" {
		return hand
	}
	return func(w http.ResponseWriter, req *http.Request) {
		h := req.Header.Get(constants.HdrRundevClientSecret)
		if h == "" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "%s header not specified", constants.HdrRundevClientSecret)
			return
		} else if subtle.ConstantTimeCompare([]byte(secret), []byte(h)) != 1 {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "client secret (%s header) on the request not matching the one configured on the daemon", constants.HdrRundevClientSecret)
			return
		}
		hand(w, req)
	}
}

// newReverseProxy returns a reverse proxy to the user’s app.
func newReverseProxy(target *url.URL) http.Handler {
	return httputil.NewSingleHostReverseProxy(target)
}

func (srv *daemonServer) reverseProxyHandler(w http.ResponseWriter, req *http.Request) {
	srv.patchLock.RLock()
	defer srv.patchLock.RUnlock()

	id := uuid.New().String()
	rr := &responseRecorder{rw: w}
	w = rr
	start := time.Now()
	log.Printf("[rev proxy] request %s accepted: path=%s method=%s", id, req.URL.Path, req.Method)
	defer func() {
		log.Printf("[rev proxy] request %s complete: path=%s status=%d took=%v", id, req.URL.Path, rr.statusCode, time.Since(start))
	}()

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

	fs, err := fsutil.Walk(srv.opts.syncDir, srv.opts.ignores)
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
		log.Printf("[rev proxy] user process not running, restarting")
		for i, bc := range srv.opts.buildCmds {
			log.Printf("[rev proxy] executing build command (%d/%d): %v", i, len(srv.opts.buildCmds), bc)
			cmd := exec.Command(bc.cmd, bc.args...)
			cmd.Dir = srv.opts.syncDir
			if b, err := cmd.CombinedOutput(); err != nil {
				srv.nannyLock.Unlock()
				log.Printf("build cmd failure: %s", string(b))
				writeProcError(w, fmt.Sprintf("executing -build-cmd (%v) failed: %s", bc, err), b)
				return
			}
			log.Print("rebuild succeeded")
		}

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

func (srv *daemonServer) patch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if ct := req.Header.Get("content-type"); ct != constants.MimePatch {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	expectedLocalChecksum := req.Header.Get(constants.HdrRundevPatchPreconditionSum)
	if expectedLocalChecksum == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "patch request did not contain %s header", constants.HdrRundevPatchPreconditionSum)
		return
	}

	incomingChecksum := req.Header.Get(constants.HdrRundevChecksum)
	if incomingChecksum == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "patch request did not contain %s header", constants.HdrRundevChecksum)
		return
	}

	// stop accepting new proxy or patch requests while potentially modifying fs
	srv.patchLock.Lock()
	defer srv.patchLock.Unlock()

	fs, err := fsutil.Walk(srv.opts.syncDir, srv.opts.ignores)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
		return
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
	log.Printf("patch applied")

	srv.nannyLock.Lock()
	srv.procNanny.Kill() // restart the process on next proxied request
	// TODO(ahmetb) ensure port goes down before calling it dead as some indirect subprocesses may exit late?
	log.Printf("existing proc killed after patch")
	srv.nannyLock.Unlock()

	w.WriteHeader(http.StatusAccepted)
	log.Printf("patch (%s) accepted", incomingChecksum)
	return
}

func (srv *daemonServer) restartHandler(w http.ResponseWriter, req *http.Request) {
	srv.nannyLock.Lock()
	defer srv.nannyLock.Unlock()

	if err := srv.procNanny.Restart(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error restarting process: %+v", err)
		return
	}
	fmt.Fprintf(w, "ok")
}

func (srv *daemonServer) killHandler(w http.ResponseWriter, req *http.Request) {
	srv.nannyLock.Lock()
	defer srv.nannyLock.Unlock()
	srv.procNanny.Kill()
}

func (srv *daemonServer) logsHandler(w http.ResponseWriter, req *http.Request) {
	srv.nannyLock.Lock()
	defer srv.nannyLock.Unlock()
	b := srv.procLogs.Bytes()
	w.Write(b)
}

func (srv *daemonServer) statusHandler(w http.ResponseWriter, req *http.Request) {
	fs, err := fsutil.Walk(srv.opts.syncDir, srv.opts.ignores)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	fmt.Fprintf(w, "fs checksum: %v\n", fs.RootChecksum())
	fmt.Fprintf(w, "pid: %d\n", os.Getpid())
	wd, _ := os.Getwd()
	fmt.Fprintf(w, "cwd: %s\n", wd)
	fmt.Fprintf(w, "child process running: %v\n", srv.procNanny.Running())
	fmt.Fprint(w, "opts:\n")
	fmt.Fprintf(w, "  ignores: %# v\n", pretty.Formatter(srv.opts.ignores))
	fmt.Fprintf(w, "  run-cmd: %# v\n", pretty.Formatter(srv.opts.runCmd))
	fmt.Fprintf(w, "  build-cmds: %# v\n", pretty.Formatter(srv.opts.buildCmds))
	fmt.Fprintf(w, "  port wait timeout: %# v\n", pretty.Formatter(srv.opts.portWaitTimeout))
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

type responseRecorder struct {
	rw         http.ResponseWriter
	statusCode int
}

func (rr *responseRecorder) Header() http.Header         { return rr.rw.Header() }
func (rr *responseRecorder) Write(b []byte) (int, error) { return rr.rw.Write(b) }
func (rr *responseRecorder) WriteHeader(statusCode int) {
	rr.statusCode = statusCode
	rr.rw.WriteHeader(statusCode)
}
