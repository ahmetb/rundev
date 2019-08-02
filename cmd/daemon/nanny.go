package main

import (
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"sync"
)

type nanny interface {
	Running() bool
	Restart() error // starts if not running
	Kill()
}

type procNanny struct {
	cmd  string
	args []string

	mu     sync.RWMutex
	proc   *os.Process
	active bool
}

func newProcessNanny(cmd string, args []string) nanny {
	return &procNanny{
		cmd:  cmd,
		args: args}
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
		p.proc.Kill()
		p.proc.Release()
	}
	p.active = false
}

func (p *procNanny) replace() error {
	p.kill()

	newProc := exec.Command(p.cmd, p.args...)
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