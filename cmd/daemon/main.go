package main

import (
	"context"
	"flag"
	"github.com/google/shlex"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

var (
	flRunCmd               string
	flBuildCmd             string
	flAddr                 string
	flSyncDir              string
	flChildPort            int
	flProcessListenTimeout time.Duration
)

func init() {
	flag.StringVar(&flSyncDir, "sync-dir", ".", "directory to sync")
	flag.StringVar(&flAddr, "addr", "localhost:8080", "network address to start the daemon") // TODO(ahmetb): make this obey $PORT
	flag.StringVar(&flBuildCmd, "build-cmd", "", "command to rebuild the user app (inside the container)")
	flag.StringVar(&flRunCmd, "run-cmd", "", "command to start the user app (inside the container)")
	flag.IntVar(&flChildPort, "user-port", 5000, "PORT value passed to the user app")
	flag.DurationVar(&flProcessListenTimeout, "process-listen-timeout", time.Second*4, "time to wait for user app to listen on PORT")
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
	if flProcessListenTimeout <= 0 {
		log.Fatal("-process-listen-timeout must be positive")
	}
	if flChildPort <= 0 || flChildPort > 65535 {
		log.Fatalf("-user-port value (%d) is invalid", flChildPort)
	}
	if flRunCmd == "" {
		log.Fatal("-run-cmd is empty")
	}
	runCmds, err := shlex.Split(flRunCmd)
	if err != nil {
		log.Fatalf("failed to parse -run-cmd: %s", err)
	} else if len(runCmds) == 0 {
		log.Fatal("-run-cmd parsed into zero tokens")
	}
	runCmd := &cmd{runCmds[0], runCmds[1:]}

	var buildCmd *cmd
	if flBuildCmd != "" {
		buildCmds, err := shlex.Split(flBuildCmd)
		if err != nil {
			log.Fatalf("failed to parse -build-cmd: %s", err)
		} else if len(buildCmds) == 0 {
			log.Fatal("-build-cmd parsed into zero tokens")
		}
		buildCmd = &cmd{buildCmds[0], buildCmds[1:]}
	}

	handler := newDaemonServer(daemonOpts{
		syncDir:         flSyncDir,
		runCmd:          runCmd,
		buildCmd:        buildCmd,
		childPort:       flChildPort,
		portWaitTimeout: flProcessListenTimeout,
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
