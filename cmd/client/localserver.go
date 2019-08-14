package main

import (
	"fmt"
	"github.com/ahmetb/rundev/lib/handlerutil"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type localServerOpts struct {
	sync        *syncer
	proxyTarget string
}

type localServer struct {
	opts localServerOpts
}

func newLocalServer(opts localServerOpts) (http.Handler, error) {
	ls := &localServer{opts: opts}

	reverseProxy, err := newReverseProxyHandler(opts.proxyTarget, ls.opts.sync)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize reverse proxy")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rundev/fsz", handlerutil.NewFSDebugHandler(ls.opts.sync.opts.localDir, ls.opts.sync.opts.ignores))
	mux.HandleFunc("/rundev/debugz", ls.debugHandler)
	mux.HandleFunc("/rundev/", handlerutil.NewUnsupportedDebugEndpointHandler())
	mux.HandleFunc("/favicon.ico", handlerutil.NewUnsupportedDebugEndpointHandler()) // TODO(ahmetb) annoyance during testing on browser
	// TODO(ahmetb) add /rundev/syncz
	mux.Handle("/", reverseProxy)
	return mux, nil
}

func newReverseProxyHandler(addr string, sync *syncer) (http.Handler, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote addr as url %s", addr)
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = withSyncingRoundTripper(rp.Transport, sync, u.Host)
	return rp, nil
}

func (srv *localServer) debugHandler(w http.ResponseWriter, req *http.Request) {
	checksum, err := srv.opts.sync.checksum()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	fmt.Fprintf(w, "fs checksum: %v\n", checksum)
	fmt.Fprintf(w, "pid: %d\n", os.Getpid())
	wd, _ := os.Getwd()
	fmt.Fprintf(w, "cwd: %s\n", wd)
	fmt.Fprint(w, "sync:\n")
	fmt.Fprintf(w, "  dir: %# v\n", pretty.Formatter(srv.opts.sync.opts.localDir))
	fmt.Fprintf(w, "  target: %# v\n", pretty.Formatter(srv.opts.sync.opts.targetAddr))
	fmt.Fprintf(w, "  ignores: %# v\n", pretty.Formatter(srv.opts.sync.opts.ignores))
}
