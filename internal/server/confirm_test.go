package server

import (
	"testing"
	"time"
)

func TestArmConfirmAutoRevertsAfterTimeout(t *testing.T) {
	s := &Server{}
	reverted := make(chan struct{}, 1)
	revert := func() error {
		reverted <- struct{}{}
		return nil
	}

	id, expires := s.armConfirm(revert, 20*time.Millisecond)
	if id == "" {
		t.Fatal("expected a non-empty confirm id")
	}
	if !expires.After(time.Now()) {
		t.Fatal("expected expires in the future")
	}

	select {
	case <-reverted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected revert to fire after timeout")
	}

	if _, _, ok := s.confirmStatus(); ok {
		t.Fatal("expected confirm to be cleared after auto-revert")
	}
}

func TestKeepConfirmCancelsAutoRevert(t *testing.T) {
	s := &Server{}
	reverted := make(chan struct{}, 1)
	revert := func() error {
		reverted <- struct{}{}
		return nil
	}

	id, _ := s.armConfirm(revert, 20*time.Millisecond)
	if !s.keepConfirm(id) {
		t.Fatal("expected keep to find the pending confirm")
	}

	select {
	case <-reverted:
		t.Fatal("revert fired after keep, expected it to be cancelled")
	case <-time.After(80 * time.Millisecond):
	}

	if _, _, ok := s.confirmStatus(); ok {
		t.Fatal("expected confirm cleared after keep")
	}
}

func TestRevertConfirmRunsImmediately(t *testing.T) {
	s := &Server{}
	reverted := make(chan struct{}, 1)
	revert := func() error {
		reverted <- struct{}{}
		return nil
	}

	id, _ := s.armConfirm(revert, time.Minute)
	if !s.revertConfirm(id) {
		t.Fatal("expected revert to find the pending confirm")
	}
	select {
	case <-reverted:
	case <-time.After(time.Second):
		t.Fatal("expected revert to run immediately")
	}
}

func TestKeepUnknownConfirmReturnsFalse(t *testing.T) {
	s := &Server{}
	if s.keepConfirm("does-not-exist") {
		t.Fatal("keep on unknown id should return false")
	}
	if s.revertConfirm("does-not-exist") {
		t.Fatal("revert on unknown id should return false")
	}
}

func TestArmConfirmIsSingleFlight(t *testing.T) {
	s := &Server{}
	firstReverted := make(chan struct{}, 1)
	secondReverted := make(chan struct{}, 1)

	firstID, _ := s.armConfirm(func() error {
		firstReverted <- struct{}{}
		return nil
	}, time.Minute)
	secondID, _ := s.armConfirm(func() error {
		secondReverted <- struct{}{}
		return nil
	}, time.Minute)

	// Only the second confirm should remain active.
	id, _, ok := s.confirmStatus()
	if !ok || id != secondID {
		t.Fatalf("expected active confirm to be %q, got %q (ok=%v)", secondID, id, ok)
	}

	// The first confirm must have been finalised as kept, never reverted.
	select {
	case <-firstReverted:
		t.Fatal("first confirm reverted, expected it to be kept by single-flight")
	case <-time.After(30 * time.Millisecond):
	}

	// Reverting the second must not touch the first.
	if !s.revertConfirm(secondID) {
		t.Fatal("expected to revert the second confirm")
	}
	select {
	case <-secondReverted:
	case <-time.After(time.Second):
		t.Fatal("expected second confirm to revert")
	}
	_ = firstID
}
