package modules

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCachedReadMissThenHit(t *testing.T) {
	InvalidateKey("test|miss-hit")
	var calls int
	compute := func() (string, error) {
		calls++
		return fmt.Sprintf("v%d", calls), nil
	}

	v, err := CachedRead("test|miss-hit", time.Minute, compute)
	if err != nil || v != "v1" {
		t.Fatalf("first call: v=%q err=%v", v, err)
	}
	v, err = CachedRead("test|miss-hit", time.Minute, compute)
	if err != nil || v != "v1" {
		t.Fatalf("cached call should reuse the first value: v=%q err=%v", v, err)
	}
	if calls != 1 {
		t.Fatalf("compute ran %d times, want 1", calls)
	}
}

func TestCachedReadExpiry(t *testing.T) {
	InvalidateKey("test|expiry")
	var calls int
	compute := func() (int, error) {
		calls++
		return calls, nil
	}

	CachedRead("test|expiry", 20*time.Millisecond, compute)
	time.Sleep(40 * time.Millisecond)
	v, _ := CachedRead("test|expiry", 20*time.Millisecond, compute)
	if v != 2 || calls != 2 {
		t.Fatalf("after ttl a fresh compute is expected: v=%d calls=%d", v, calls)
	}
}

func TestCachedReadErrorNotCachedForever(t *testing.T) {
	InvalidateKey("test|err")
	var calls int
	compute := func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("boom")
		}
		return "ok", nil
	}

	if _, err := CachedRead("test|err", 20*time.Millisecond, compute); err == nil {
		t.Fatal("first call should fail")
	}
	time.Sleep(40 * time.Millisecond)
	v, err := CachedRead("test|err", 20*time.Millisecond, compute)
	if err != nil || v != "ok" {
		t.Fatalf("after ttl the error may be retried: v=%q err=%v", v, err)
	}
}

func TestInvalidateKey(t *testing.T) {
	InvalidateKey("test|inv")
	var calls int
	compute := func() (string, error) { calls++; return "x", nil }

	CachedRead("test|inv", time.Minute, compute)
	InvalidateKey("test|inv")
	CachedRead("test|inv", time.Minute, compute)
	if calls != 2 {
		t.Fatalf("invalidate should force a recompute, calls=%d", calls)
	}

	// Invalidating an absent key must not panic.
	InvalidateKey("test|never-cached")
}

func TestInvalidatePrefix(t *testing.T) {
	InvalidatePrefix("test|pfx|")
	var calls int
	compute := func() (string, error) { calls++; return "x", nil }

	CachedRead("test|pfx|a", time.Minute, compute)
	CachedRead("test|pfx|b", time.Minute, compute)
	CachedRead("test|pfx-other", time.Minute, compute)

	InvalidatePrefix("test|pfx|")

	CachedRead("test|pfx|a", time.Minute, compute)
	CachedRead("test|pfx|b", time.Minute, compute)
	CachedRead("test|pfx-other", time.Minute, compute)
	if calls != 5 {
		t.Fatalf("prefix invalidation should recompute pfx|a and pfx|b only, calls=%d", calls)
	}
}

func TestCachedReadConcurrentSingleflight(t *testing.T) {
	InvalidateKey("test|race")
	var calls atomic.Int32
	compute := func() (int, error) {
		calls.Add(1)
		time.Sleep(30 * time.Millisecond)
		return 42, nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := CachedRead("test|race", time.Minute, compute)
			if err != nil || v != 42 {
				t.Errorf("concurrent read: v=%d err=%v", v, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent callers of one key must compute once, got %d", calls.Load())
	}
}

func TestUciShowCached(t *testing.T) {
	InvalidateKey("ucishow|netgrip-readcache-test")
	// uci is not expected to exist in the test environment; both outcomes
	// are fine as long as the memoization contract holds.
	first, firstOK := uciShowCached("netgrip-readcache-test")
	second, secondOK := uciShowCached("netgrip-readcache-test")
	if firstOK != secondOK || first != second {
		t.Fatalf("second read should be served from cache: %q/%v vs %q/%v", first, firstOK, second, secondOK)
	}
	InvalidateKey("ucishow|netgrip-readcache-test")
}
