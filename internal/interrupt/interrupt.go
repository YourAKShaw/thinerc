// Copyright 2026 Ayush Kumar Shaw
// SPDX-License-Identifier: Apache-2.0

// Package interrupt decides what a termination signal means right now (S10).
// Outside a protected section it ends the process at once. Inside one, the
// first signal cancels a context so the work can stop at a safe point, and a
// second signal ends the process anyway.
package interrupt

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
)

var depth atomic.Int32

// Protect marks the start of work that must not be cut off halfway. Call the
// returned function when that work and its bookkeeping are finished.
func Protect() (release func()) {
	depth.Add(1)
	var once sync.Once
	return func() { once.Do(func() { depth.Add(-1) }) }
}

// Watch consumes signals from sig and returns a context cancelled by the first
// signal that arrives during a protected section. exit must end the process.
func Watch(parent context.Context, sig <-chan os.Signal, exit func()) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		<-sig
		if depth.Load() == 0 {
			exit()
			return
		}
		cancel()
		<-sig
		exit()
	}()
	return ctx
}
