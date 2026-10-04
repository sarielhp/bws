package main

import (
	"sync/atomic"
	"testing"
	"time"

	"bws/internal/dbus"
)

func TestSandboxSignalLifecycleCleanExit(t *testing.T) {
	cleanupCalled := int32(0)
	cleanup := func() {
		atomic.AddInt32(&cleanupCalled, 1)
	}

	var dbusProxy atomic.Pointer[dbus.Proxy]
	safeCleanup := startSandboxSignalMonitor(cleanup, func() *dbus.Proxy {
		return dbusProxy.Load()
	})

	if safeCleanup == nil {
		t.Fatal("expected non-nil safeCleanup")
	}

	safeCleanup()

	if val := atomic.LoadInt32(&cleanupCalled); val != 1 {
		t.Fatalf("expected cleanup called 1 time, got %d", val)
	}

	safeCleanup()
	if val := atomic.LoadInt32(&cleanupCalled); val != 1 {
		t.Fatalf("expected cleanup to remain called 1 time (idempotent), got %d", val)
	}
}

func TestSandboxDBusAtomicConcurrency(t *testing.T) {
	var dbusProxy atomic.Pointer[dbus.Proxy]
	stop := make(chan struct{})

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			select {
			case <-stop:
				return
			default:
				_ = dbusProxy.Load()
				time.Sleep(10 * time.Microsecond)
			}
		}
	}()

	for i := 0; i < 100; i++ {
		dbusProxy.Store(nil)
		time.Sleep(10 * time.Microsecond)
	}

	close(stop)
	<-readerDone
}
