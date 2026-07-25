// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/device"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/preflight"
)

type App struct {
	ctx context.Context
}

type Candidate struct {
	Address       string `json:"address"`
	Banner        string `json:"banner"`
	DeveloperMode bool   `json:"developerMode"`
	USB           bool   `json:"usb"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Discover performs the same USB-first/dropbear probe used by remagic. It is
// read-only and does not authenticate or change the device.
func (a *App) Discover() []Candidate {
	probes := device.Discover(650 * time.Millisecond)
	if len(probes) == 0 {
		probes = device.Discover(2 * time.Second)
	}
	candidates := make([]Candidate, 0, len(probes))
	for _, probe := range probes {
		candidates = append(candidates, Candidate{
			Address:       probe.Addr,
			Banner:        probe.Banner,
			DeveloperMode: probe.IsPaperPro(),
			USB:           probe.Addr == device.DefaultUSBAddr,
		})
	}
	return candidates
}

// Inspect authenticates only long enough to run an allowlisted, read-only
// preflight. Passwords are held in memory for this call and are never logged.
func (a *App) Inspect(host, password string) (preflight.Status, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		host = device.DefaultUSBAddr
	}
	connection, err := device.Connect(host, strings.TrimSpace(password))
	if err != nil {
		return preflight.Status{}, fmt.Errorf("connect to Move: %w", err)
	}
	defer connection.Close()
	return preflight.Inspect(connection, host)
}

func (a *App) ImplementationState() map[string]bool {
	return map[string]bool{
		"discovery": true,
		"preflight": true,
		"install":   false,
		"update":    false,
		"repair":    false,
		"login":     false,
		"uninstall": false,
	}
}
