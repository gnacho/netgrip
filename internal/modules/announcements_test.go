package modules

import (
	"testing"
	"time"
)

func TestActiveAnnouncementWindow(t *testing.T) {
	mk := func(id, starts, expires string) Announcement {
		return Announcement{ID: id, Starts: starts, Expires: expires}
	}
	now := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	list := []Announcement{
		mk("expired", "", "2026-09-01"),
		mk("future", "2026-10-01", ""),
		mk("valid", "2026-09-01", "2026-11-01"),
		mk("no-dates", "", ""),
	}
	got := activeAnnouncement(list, now)
	if got == nil || got.ID != "valid" {
		t.Fatalf("expected valid, got %+v", got)
	}
	allExpired := activeAnnouncement(list[:2], now)
	if allExpired != nil {
		t.Fatalf("expected nil, got %+v", allExpired)
	}
	if got := activeAnnouncement([]Announcement{{Starts: "", Expires: ""}}, now); got != nil {
		t.Fatalf("empty id must be skipped, got %+v", got)
	}
}
