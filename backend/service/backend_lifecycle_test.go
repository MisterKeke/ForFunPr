package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceShutdownCancelsAndWaitsForActiveOperations(t *testing.T) {
	service := newFeatureTestService(t)
	operationContext, done, err := service.BeginOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	shutdownDone := make(chan struct{})
	go func() {
		service.Shutdown(context.Background())
		close(shutdownDone)
	}()

	select {
	case <-operationContext.Done():
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel the active operation context")
	}
	select {
	case <-shutdownDone:
		t.Fatal("shutdown returned before the active operation released its pin")
	default:
	}

	// The completion function is intentionally safe for adapter cleanup paths
	// that may invoke it more than once.
	done()
	done()
	select {
	case <-shutdownDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after the active operation completed")
	}
	if service.GetStartupStatus().Ready {
		t.Fatal("service remained ready after shutdown")
	}
	if _, _, err := service.BeginOperation(context.Background()); !errors.Is(err, ErrBackendNotReady) {
		t.Fatalf("BeginOperation after shutdown = %v, want ErrBackendNotReady", err)
	}
}

func TestServiceShutdownHonorsCallerCancellationWithoutClosingInUseDatabase(t *testing.T) {
	service := newFeatureTestService(t)
	_, done, err := service.BeginOperation(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	returned := make(chan struct{})
	go func() {
		service.Shutdown(ctx)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("shutdown ignored its canceled context")
	}

	service.lifecycleMu.Lock()
	databaseStillOpen := service.db != nil
	service.lifecycleMu.Unlock()
	if !databaseStillOpen {
		t.Fatal("shutdown closed the database while an operation still held a lifecycle pin")
	}

	done()
	deadline := time.Now().Add(time.Second)
	for {
		service.lifecycleMu.Lock()
		closed := service.db == nil
		service.lifecycleMu.Unlock()
		if closed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("deferred shutdown cleanup did not close the database")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestServiceStartupErrorCancelsOperationsAndBlocksNewWork(t *testing.T) {
	service := newFeatureTestService(t)
	operationContext, done, err := service.BeginOperation(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer done()

	service.SetStartupError(errors.New("startup failed"))
	select {
	case <-operationContext.Done():
	case <-time.After(time.Second):
		t.Fatal("startup failure did not cancel active operation context")
	}
	if status := service.GetStartupStatus(); status.Ready || status.Error == "" {
		t.Fatalf("startup status = %#v", status)
	}
	if _, _, err := service.BeginOperation(nil); !errors.Is(err, ErrBackendNotReady) {
		t.Fatalf("BeginOperation after startup error = %v, want ErrBackendNotReady", err)
	}
}

func TestStartupCapabilitiesIncludeRunningApps(t *testing.T) {
	states := defaultCapabilityStates()
	if _, ok := states["running_apps"]; !ok {
		t.Fatal("default startup capabilities do not include running_apps")
	}
	if !capabilityNameExists("running_apps") {
		t.Fatal("capabilityNames does not include running_apps")
	}
}

func capabilityNameExists(name string) bool {
	for _, candidate := range capabilityNames() {
		if candidate == name {
			return true
		}
	}
	return false
}
