// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"net"
	"time"
)

const (
	defaultPortRetryInterval = time.Millisecond * 5
	defaultPortDialTimeout   = time.Millisecond * 40
)

type portChecker interface {
	checkPort() bool
	waitPort(context.Context) error
}

type tcpPortCheck struct {
	portNum       int
	retryInterval time.Duration
	dialTimeout   time.Duration
}

func newTCPPortChecker(port int) portChecker {
	return &tcpPortCheck{
		portNum:       port,
		retryInterval: defaultPortRetryInterval,
		dialTimeout:   defaultPortDialTimeout}
}

func (t *tcpPortCheck) checkPort() bool {
	addr := net.JoinHostPort("localhost", fmt.Sprintf("%d", t.portNum))
	conn, err := net.DialTimeout("tcp", addr, t.dialTimeout)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	return err == nil
}

// waitPort waits for port to be connectable until the specified ctx is cancelled.
func (t *tcpPortCheck) waitPort(ctx context.Context) error {
	// TODO: do we need to return error from this method?
	ch := make(chan struct{}, 1)
	defer close(ch)

	tick := time.NewTicker(t.retryInterval)
	defer tick.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if ok := t.checkPort(); ok {
					ch <- struct{}{}
					return
				}
				time.Sleep(time.Millisecond * 10)
			}
		}
	}()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "quit waiting on port to open")
	}
}
