package main

import (
	"context"
	"testing"
	"time"
)

func TestExecFails(t *testing.T) {
	n := newProcessNanny("non-existing", nil, procOpts{})
	err := n.Restart()
	defer n.Kill()
	if err == nil {
		t.Fatal("no error?")
	}
	if n.Running() {
		t.Fatal("should not be running")
	}
}

func TestExec(t *testing.T) {
	n := newProcessNanny("sleep", []string{"100"}, procOpts{})
	defer n.Kill()
	if n.Running() {
		t.Fatal("not started yet")
	}
	err := n.Restart()
	if err != nil {
		t.Fatal(err)
	}
	if !n.Running() {
		t.Fatal("not running")
	}
	n.Kill()
	time.Sleep(time.Millisecond * 100) // replace with a wait loop
	if n.Running() {
		t.Fatal("killed process still running")
	}
}

func TestExecReplaceRunning(t *testing.T) {
	n := newProcessNanny("sleep", []string{"1000"}, procOpts{})
	defer n.Kill()
	if err := n.Restart(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := n.Restart(); err != nil {
			t.Fatal(err)
		}
	}
	if !n.Running() {
		t.Fatal("not running")
	}
}

func TestExecCapturesExit(t *testing.T) {
	n := newProcessNanny("sleep", []string{"0.1"}, procOpts{})
	defer n.Kill()

	if err := n.Restart(); err != nil {
		t.Fatal()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*300)
	defer cancel()
	i := 0
	for n.Running() {
		select {
		case <-ctx.Done():
			t.Fatalf("cmd did not terminate, iteration:%d", i)
		default:
			i++
		}
		time.Sleep(time.Millisecond * 5)
	}
}
