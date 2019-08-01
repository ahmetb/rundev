package main

import (
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/pkg/errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type syncOpts struct {
	localDir string
}

type localServerOpts struct {
	targetAddr string
	sync       syncOpts
}

type localServer struct {
	opts localServerOpts
	sync *syncer
}

func newLocalServer(opts localServerOpts) (http.Handler, error) {
	ls := &localServer{
		opts: opts,
		sync: newSyncer(opts.sync)}

	reverseProxy, err := newReverseProxyHandler(opts.targetAddr, ls.sync)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize reverse proxy")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rundev/fsz", ls.fsHandler)
	mux.Handle("/", reverseProxy)
	return mux, nil
}

func newReverseProxyHandler(addr string, sync *syncer) (http.Handler, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote addr as url %s", addr)
	}
	rp := httputil.NewSingleHostReverseProxy(u)
	rp.Transport = withSyncingRoundTripper(rp.Transport, sync)
	return rp, nil
}

func (ls *localServer) fsHandler(w http.ResponseWriter, req *http.Request) {
	fs, err := fsutil.Walk(ls.opts.sync.localDir)
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
