package main

import (
	"encoding/json"
	"fmt"
	"github.com/ahmetb/rundev/lib/constants"
	"github.com/ahmetb/rundev/lib/fsutil"
	"github.com/pkg/errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	mux.HandleFunc("/rundev/fsz", ls.fsHandler)
	mux.HandleFunc("/rundev/debugz", ls.debugHandler)
	mux.HandleFunc("/rundev/", ls.unsupported)     // prevent proxying client debug endpoints
	mux.HandleFunc("/favicon.ico", ls.unsupported) // TODO(ahmetb) annoyance during testing on browser
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
	rp.Transport = withSyncingRoundTripper(rp.Transport, sync)
	return rp, nil
}

func (srv *localServer) fsHandler(w http.ResponseWriter, req *http.Request) {
	fs, err := fsutil.Walk(srv.opts.sync.opts.localDir)
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

func (srv *localServer) debugHandler(w http.ResponseWriter, req *http.Request) {
	checksum, err := srv.opts.sync.checksum()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Errorf("failed to fetch local filesystem: %+v", err)
	}
	fmt.Fprintf(w, "fs checksum: %v\n", checksum)
	fmt.Fprintf(w, "opts: %#v\n", srv.opts)
	fmt.Fprintf(w, "sync opts: %#v\n", srv.opts.sync.opts)
}

func (*localServer) unsupported(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, "unsupported rundev client endpoint %s", req.URL.Path)
}
