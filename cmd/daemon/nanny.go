package main

import (
	"bytes"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
)

type nanny interface {
	Running() bool
	Restart() error // starts if not running
	Kill()
}

type procNanny struct {
	cmd  string
	args []string
	opts procOpts

	mu     sync.RWMutex
	proc   *os.Process
	active bool
}

type procOpts struct {
	port int
	dir  string
	logs *bytes.Buffer
}

func newProcessNanny(cmd string, args []string, opts procOpts) nanny {
	return &procNanny{
		cmd:  cmd,
		args: args,
		opts: opts,
	}
}

func (p *procNanny) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.active
}

func (p *procNanny) Kill() {
	p.kill()
}

func (p *procNanny) Restart() error {
	return p.replace()
}

// kill sends a SIGKILL to the process if it's running.
func (p *procNanny) kill() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.proc != nil {
		pid := -p.proc.Pid  // negative value: ID of process group
		log.Printf("killing pid %d", pid)
		// TODO using negative PID (pgrp kill) not working on gVisor
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			log.Printf("warning: failed to kill: %v", err)
		} else {
			log.Printf("killed pid %d", pid)
		}

		//// TODO until we can get negative PID kills working, kill every process except self and 1
		//ps, _ := pstree.New()
		//for pp := range ps.Procs {
		//	if pp != os.Getpid() && pp != 1 {
		//		log.Printf("found pid=%d, killing", pp)
		//		syscall.Kill(pp, syscall.SIGKILL)
		//	}
		//}
		p.proc.Release()
		p.proc = nil
	}
	p.active = false
	if p.opts.logs != nil {
		p.opts.logs.Reset()
	}
}

func (p *procNanny) replace() error {
	p.kill()

	newProc := exec.Command(p.cmd, p.args...)
	newProc.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true} // create a new GID
	if p.opts.dir != "" {
		newProc.Dir = p.opts.dir
	}
	newProc.Stdout = io.MultiWriter(p.opts.logs, os.Stdout)
	newProc.Stderr = io.MultiWriter(p.opts.logs, os.Stderr)

	if p.opts.port > 0 {
		newProc.Env = append(os.Environ(), "PORT="+strconv.Itoa(p.opts.port))
	}
	log.Printf("proc start")
	if err := newProc.Start(); err != nil {
		return errors.Wrap(err, "error starting process")
	}

	p.mu.Lock()
	p.proc = newProc.Process
	p.active = true
	p.mu.Unlock()

	go func(origProc *os.Process) {
		_ = newProc.Wait()
		p.mu.Lock()
		if p.proc == origProc {
			p.active = false
		}
		p.mu.Unlock()
	}(newProc.Process)

	return nil
}
