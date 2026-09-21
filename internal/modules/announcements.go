// announcements.go — avisos externos para todas las instalaciones: el server
// consulta announcements.json del repo (raw de GitHub, como hace el updater)
// y expone el aviso vigente en /api/announcement. La UI lo pinta como una
// franja descartable en el shell. Override de la fuente con
// NETGRIP_ANNOUNCEMENTS_URL (pruebas); fail-silent: sin red, sin aviso.
package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	defaultAnnouncementsURL = "https://raw.githubusercontent.com/gnacho/netgrip/main/announcements.json"
	announcementsRefresh    = 6 * time.Hour
	announcementsTimeout    = 5 * time.Second
	announcementsMaxBytes   = 64 << 10
)

// Announcement es un aviso publicado en announcements.json. Title/Body van
// por idioma (es/en; el cliente cae a en). Starts/Expires ("YYYY-MM-DD",
// opcionales) delimitan la ventana de vigencia.
type Announcement struct {
	ID       string            `json:"id"`
	Urgency  string            `json:"urgency"` // "info" | "warn"
	Title    map[string]string `json:"title"`
	Body     map[string]string `json:"body,omitempty"`
	URL      string            `json:"url,omitempty"`
	URLLabel map[string]string `json:"urlLabel,omitempty"`
	Starts   string            `json:"starts,omitempty"`
	Expires  string            `json:"expires,omitempty"`
}

type announcementsFile struct {
	Announcements []Announcement `json:"announcements"`
}

var announcementsCache = struct {
	mu  sync.Mutex
	cur *Announcement
}{}

func setActiveAnnouncement(a *Announcement) {
	announcementsCache.mu.Lock()
	defer announcementsCache.mu.Unlock()
	announcementsCache.cur = a
}

// GetActiveAnnouncement devuelve el aviso vigente cacheado, o nil.
func GetActiveAnnouncement() *Announcement {
	announcementsCache.mu.Lock()
	defer announcementsCache.mu.Unlock()
	return announcementsCache.cur
}

// activeAnnouncement elige el primer aviso vigente (starts <= hoy < expires).
func activeAnnouncement(list []Announcement, now time.Time) *Announcement {
	today := now.Format("2006-01-02")
	for i := range list {
		a := list[i]
		if a.ID == "" {
			continue
		}
		if a.Starts != "" && today < a.Starts {
			continue
		}
		if a.Expires != "" && today >= a.Expires {
			continue
		}
		return &a
	}
	return nil
}

func fetchAnnouncements(ctx context.Context, url string) (*Announcement, error) {
	ctx, cancel := context.WithTimeout(ctx, announcementsTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "netgrip-announcements")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("announcements: HTTP %d", res.StatusCode)
	}
	var file announcementsFile
	if err := json.NewDecoder(io.LimitReader(res.Body, announcementsMaxBytes)).Decode(&file); err != nil {
		return nil, err
	}
	return activeAnnouncement(file.Announcements, time.Now()), nil
}

// StartAnnouncements lanza el bucle de refresco (daemon; vive con el server).
func StartAnnouncements() {
	url := defaultAnnouncementsURL
	if v := os.Getenv("NETGRIP_ANNOUNCEMENTS_URL"); v != "" {
		url = v
	}
	refresh := func() {
		a, err := fetchAnnouncements(context.Background(), url)
		if err != nil {
			log.Printf("announcements: fetch failed: %v", err)
			return
		}
		setActiveAnnouncement(a)
	}
	go func() {
		refresh()
		t := time.NewTicker(announcementsRefresh)
		defer t.Stop()
		for range t.C {
			refresh()
		}
	}()
}
