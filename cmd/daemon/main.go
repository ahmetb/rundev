package main

import (
	"context"
	"flag"
	"github.com/google/shlex"
	"log"
	"net/http"
	"os"
	"os/signal"
)

var (
	flRunCmd    string
	flBuildCmd  string
	flAddr      string
	flSyncDir   string
	flChildPort int
)

func init() {
	flag.StringVar(&flSyncDir, "sync-dir", ".", "directory to sync")
	flag.StringVar(&flAddr, "addr", "localhost:8080", "network address to start the daemon") // TODO(ahmetb): make this obey $PORT
	flag.StringVar(&flBuildCmd, "build-cmd", "", "command to rebuild the user app (inside the container)")
	flag.StringVar(&flRunCmd, "run-cmd", "", "command to start the user app (inside the container)")
	flag.IntVar(&flChildPort, "user-port", 8081, "PORT value passed to the user app")
	flag.Parse()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	go func() {
		sig := <-signalCh
		log.Printf("termination signal received: %s", sig)
		cancel()
	}()

	if flSyncDir == "" {
		log.Fatal("-sync-dir is empty")
	}
	if flAddr == "" {
		log.Fatal("-addr is empty")
	}
	if flRunCmd == "" {
		log.Fatal("-run-cmd is empty")
	}
	if flChildPort <= 0 || flChildPort > 65535 {
		log.Fatalf("-user-port value (%d) is invalid", flChildPort)
	}
	runCmds, err := shlex.Split(flRunCmd)
	if err != nil {
		log.Fatalf("failed to parse -run-cmd: %s", err)
	} else if len(runCmds) == 0 {
		log.Fatal("-run-cmd parsed into zero tokens")
	}
	runCmd, runCmdArgs := runCmds[0], runCmds[1:]

	handler := newDaemonServer(daemonOpts{
		syncDir:   flSyncDir,
		runCmd:    cmd{runCmd, runCmdArgs},
		childPort: flChildPort,
	})

	localServer := http.Server{
		Handler: handler,
		Addr:    flAddr}
	go func() {
		<-ctx.Done()
		log.Println("shutting down daemon server")
		localServer.Shutdown(ctx)
	}()
	log.Printf("daemon server starting at %s", flAddr)
	if err := localServer.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			log.Printf("local server shut down gracefully, exiting")
			os.Exit(0)
		}
		log.Fatalf("local server failed to start: %+v", err)
	}
}
