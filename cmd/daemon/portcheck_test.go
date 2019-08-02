package main

import (
	"context"
	"github.com/pkg/errors"
	"net"
	"testing"
	"time"
)

func Test_checkPortOpen(t *testing.T) {
	ok := newTCPPortChecker().checkPort(9999)
	if ok {
		t.Fatal("port should not be detected as open")
	}

	li, err := net.Listen("tcp", "localhost:56771")
	if err != nil {
		t.Fatal(err)
	}
	defer li.Close()

	ok = newTCPPortChecker().checkPort(56771)
	if !ok {
		t.Fatal("port should be detected as open")
	}
}

func Test_waitPort(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()
	err := newTCPPortChecker().waitPort(ctx, 9999)
	if err == nil {
		t.Fatal("should've gotten a context cancellation error")
	}
	underlying := errors.Cause(err)
	if underlying != context.DeadlineExceeded {
		t.Fatalf("inner error is not timeline exceeded: %+v", err)
	}

	li, err := net.Listen("tcp", "localhost:56771")
	if err != nil {
		t.Fatal(err)
	}
	defer li.Close()

	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel2()
	err = newTCPPortChecker().waitPort(ctx2, 56771)
	if err != nil {
		t.Fatalf("got error from open port: %v", err)
	}
}
