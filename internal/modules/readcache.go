package modules

import (
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Read cache for the polled probes. The Overview fires ~16 GETs in
// parallel every few seconds and each probe forks busybox several times;
// on a mipsle router that saturated the CPU and the page took 2-3s (#356).
// CachedRead dedupes concurrent computes of the same key (singleflight per
// key) and serves fresh results within ttl.
//
// Writes invalidate explicitly via InvalidateKey/InvalidatePrefix so data
// is never stale for more than a couple of seconds after a change. A key
// must always be computed with the same type T (the entry is stored as
// any and type-asserted on hit).

var (
	readCacheMu      sync.Mutex
	readCacheEntries = map[string]*readCacheEntry{}
)

type readCacheEntry struct {
	mu    sync.Mutex // serialises computes of this key: one fork storm at a time
	at    time.Time
	value any
	err   error
}

// CachedRead returns the cached value for key when it is younger than ttl,
// otherwise it computes, stores and returns the fresh value. Concurrent
// callers of the same key wait on each other: the first compute wins and
// the rest get its result, so a burst of parallel polls forks once.
func CachedRead[T any](key string, ttl time.Duration, compute func() (T, error)) (T, error) {
	readCacheMu.Lock()
	e, ok := readCacheEntries[key]
	if !ok {
		e = &readCacheEntry{}
		readCacheEntries[key] = e
	}
	readCacheMu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.value != nil && time.Since(e.at) < ttl {
		return e.value.(T), e.err
	}
	value, err := compute()
	e.value, e.err, e.at = value, err, time.Now()
	return value, err
}

// InvalidateKey drops one cached entry (no-op when absent).
func InvalidateKey(key string) {
	readCacheMu.Lock()
	delete(readCacheEntries, key)
	readCacheMu.Unlock()
}

// InvalidatePrefix drops every cached entry whose key starts with prefix.
func InvalidatePrefix(prefix string) {
	readCacheMu.Lock()
	for k := range readCacheEntries {
		if strings.HasPrefix(k, prefix) {
			delete(readCacheEntries, k)
		}
	}
	readCacheMu.Unlock()
}

// uciShowTTL memoizes one `uci show <pkg>` fork. Several probes parse the
// same package (clients and wireless both read "wireless", portforward and
// the block list both read "firewall") and the Overview polls them at the
// same time; the second read would fork the same show for nothing.
const uciShowTTL = 2 * time.Second

// uciShowCached returns the output of `uci show <pkg>`, memoized for
// uciShowTTL. Writes that commit the package must InvalidateKey the
// "ucishow|" + pkg entry. ok is false when the command fails.
func uciShowCached(pkg string) (string, bool) {
	out, err := CachedRead("ucishow|"+pkg, uciShowTTL, func() (string, error) {
		b, err := exec.Command("uci", "show", pkg).Output()
		return string(b), err
	})
	if err != nil {
		return "", false
	}
	return out, true
}
