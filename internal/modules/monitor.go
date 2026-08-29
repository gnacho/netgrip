package modules

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gnacho/netgrip/internal/ubus"
)

const monitorInterval = 15 * time.Second

type Monitor struct {
	mu        sync.Mutex
	wanUp     *bool
	knownMACs map[string]bool
	stopCh    chan struct{}
}

var monitorInstance *Monitor

func StartMonitor() {
	if monitorInstance != nil {
		return
	}
	m := &Monitor{
		knownMACs: make(map[string]bool),
		stopCh:    make(chan struct{}),
	}
	monitorInstance = m
	go m.run()
}

func StopMonitor() {
	if monitorInstance != nil {
		close(monitorInstance.stopCh)
		monitorInstance = nil
	}
}

func (m *Monitor) run() {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *Monitor) check() {
	cfg := LoadTelegramConfig()
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChatID == "" {
		return
	}

	m.checkWan(cfg)
	m.checkNewClients(cfg)
}

func (m *Monitor) checkWan(cfg TelegramConfig) {
	wan, err := ubus.GetWanStatus()
	if err != nil {
		return
	}
	isUp := wan.Up

	m.mu.Lock()
	prev := m.wanUp
	m.wanUp = &isUp
	m.mu.Unlock()

	if prev == nil {
		return
	}

	wasUp := *prev
	if isUp == wasUp {
		return
	}

	if isUp {
		text := fmt.Sprintf("✅ <b>NetGrip</b>\n🌐 WAN recovered — Internet connection restored")
		if err := SendTelegram(cfg, text, true); err != nil {
			log.Printf("[monitor] telegram WAN up: %v", err)
		}
	} else {
		text := fmt.Sprintf("🔴 <b>NetGrip</b>\n🌐 WAN down — Internet connection lost")
		if err := SendTelegram(cfg, text, true); err != nil {
			log.Printf("[monitor] telegram WAN down: %v", err)
		}
	}
}

func (m *Monitor) checkNewClients(cfg TelegramConfig) {
	clients := ListClients("")

	currentMACs := make(map[string]bool, len(clients))
	for _, c := range clients {
		if c.MAC != "" {
			currentMACs[c.MAC] = true
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.knownMACs) == 0 {
		m.knownMACs = currentMACs
		return
	}

	var newOnes []Client
	for _, c := range clients {
		if c.MAC != "" && !m.knownMACs[c.MAC] {
			newOnes = append(newOnes, c)
		}
	}

	m.knownMACs = currentMACs

	if len(newOnes) == 0 {
		return
	}

	for _, c := range newOnes {
		name := c.Name
		if name == "" {
			name = "Unknown device"
		}
		text := fmt.Sprintf("📱 <b>NetGrip</b>\nNew client: <b>%s</b>\nMAC: %s · %s",
			htmlEsc(name), c.MAC, c.Type)
		if err := SendTelegram(cfg, text, false); err != nil {
			log.Printf("[monitor] telegram new client: %v", err)
		}
	}
}
