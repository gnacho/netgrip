package modules

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNtfyConfigRoundtrip(t *testing.T) {
	orig := ntfyConfigPath
	ntfyConfigPath = filepath.Join(t.TempDir(), "ntfy.json")
	t.Cleanup(func() { ntfyConfigPath = orig })

	cfg := NtfyConfig{Server: "https://ntfy.example.com/", Topic: "my-topic", Token: "secret", Enabled: true}
	if err := SaveNtfyConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := LoadNtfyConfig()
	if got.Server != "https://ntfy.example.com" {
		t.Errorf("server = %q, want trailing slash trimmed", got.Server)
	}
	if got.Topic != "my-topic" || got.Token != "secret" || !got.Enabled {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestNtfySaveValidation(t *testing.T) {
	orig := ntfyConfigPath
	ntfyConfigPath = filepath.Join(t.TempDir(), "ntfy.json")
	t.Cleanup(func() { ntfyConfigPath = orig })

	cases := []struct {
		cfg  NtfyConfig
		fail bool
	}{
		{NtfyConfig{Server: "https://ntfy.sh", Topic: "ok_1"}, false},
		{NtfyConfig{Server: "https://ntfy.sh", Topic: "a-b_C9"}, false},
		{NtfyConfig{Server: "", Topic: "t"}, false},                                   // empty server -> default
		{NtfyConfig{Server: "ftp://x", Topic: "t"}, true},                             // not http(s)
		{NtfyConfig{Server: "https://ntfy.sh", Topic: "bad topic"}, true},             // space
		{NtfyConfig{Server: "https://ntfy.sh", Topic: "a/b"}, true},                   // slash
		{NtfyConfig{Server: "https://ntfy.sh", Topic: strings.Repeat("a", 65)}, true}, // too long
		{NtfyConfig{Server: "https://ntfy.sh", Topic: ""}, false},                     // empty = clear
	}
	for _, c := range cases {
		err := SaveNtfyConfig(c.cfg)
		if c.fail && err == nil {
			t.Errorf("cfg %+v: expected error", c.cfg)
		}
		if !c.fail && err != nil {
			t.Errorf("cfg %+v: unexpected error %v", c.cfg, err)
		}
	}
}

func TestValidNtfyTopic(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"", false},
		{"abc", true},
		{"ABC123-_", true},
		{"a b", false},
		{"a/b", false},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
	} {
		if got := validNtfyTopic(tc.in); got != tc.want {
			t.Errorf("validNtfyTopic(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSendNtfyPublish(t *testing.T) {
	var requests int32
	var gotTitle, gotPriority, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		gotTitle = r.Header.Get("Title")
		gotPriority = r.Header.Get("Priority")
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/mytopic" {
			t.Errorf("path = %q, want /mytopic", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := NtfyConfig{Server: srv.URL, Topic: "mytopic", Token: "sekret", Enabled: true}
	if err := SendNtfy(cfg, "NetGrip", "hello", true); err != nil {
		t.Fatalf("SendNtfy: %v", err)
	}
	if atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("requests = %d, want 1", atomic.LoadInt32(&requests))
	}
	if gotTitle != "NetGrip" || gotPriority != "urgent" || gotAuth != "Bearer sekret" {
		t.Errorf("headers = title %q priority %q auth %q", gotTitle, gotPriority, gotAuth)
	}
}

func TestSendNtfyNoRetryOn4xx(t *testing.T) {
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	cfg := NtfyConfig{Server: srv.URL, Topic: "mytopic", Enabled: true}
	err := SendNtfy(cfg, "NetGrip", "hello", false)
	if err == nil {
		t.Fatal("expected error on 403")
	}
	if atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("requests = %d, want 1 (no retry on 4xx)", atomic.LoadInt32(&requests))
	}
}

func TestSendNtfyRetryOn5xx(t *testing.T) {
	var requests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := NtfyConfig{Server: srv.URL, Topic: "mytopic", Enabled: true}
	err := SendNtfy(cfg, "NetGrip", "hello", false)
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if atomic.LoadInt32(&requests) != 2 {
		t.Fatalf("requests = %d, want 2 (retry on 5xx)", atomic.LoadInt32(&requests))
	}
}

func TestSendNtfyErrorRedactsTopic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("boom"))
	}))
	t.Cleanup(srv.Close)

	cfg := NtfyConfig{Server: srv.URL, Topic: "super-secret-topic", Enabled: true}
	err := SendNtfy(cfg, "NetGrip", "hello", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super-secret-topic") {
		t.Fatalf("error leaks topic: %q", err.Error())
	}
}

func TestSendNtfyErrorRedactsTopicOnTransportError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listening: connection refused, URL contains the topic

	cfg := NtfyConfig{Server: "http://" + addr, Topic: "super-secret-topic", Enabled: true}
	sendErr := SendNtfy(cfg, "NetGrip", "hello", false)
	if sendErr == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(sendErr.Error(), "super-secret-topic") {
		t.Fatalf("error leaks topic: %q", sendErr.Error())
	}
}
