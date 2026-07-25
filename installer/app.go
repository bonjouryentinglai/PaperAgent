// SPDX-License-Identifier: MIT
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bonjouryentinglai/paper-agent/installer/internal/device"
	"github.com/bonjouryentinglai/paper-agent/installer/internal/maintenance"
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

type OperationResult struct {
	Status  preflight.Status `json:"status"`
	Summary string           `json:"summary"`
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

// Uninstall removes Paper Agent only. Shared XOVI/AppLoad components, the
// Node/Pi runtime, ChatGPT credentials, user configuration, and backups remain
// on the Move. The explicit acknowledgement is also enforced in the backend so
// a frontend bug cannot bypass it.
func (a *App) Uninstall(host, password string, acknowledged bool) (OperationResult, error) {
	if !acknowledged {
		return OperationResult{}, fmt.Errorf("confirm the Paper Agent-only uninstall first")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = device.DefaultUSBAddr
	}
	connection, err := device.Connect(host, strings.TrimSpace(password))
	if err != nil {
		return OperationResult{}, fmt.Errorf("connect to Move: %w", err)
	}
	defer connection.Close()

	before, err := preflight.Inspect(connection, host)
	if err != nil {
		return OperationResult{}, err
	}
	if !before.SupportedModel {
		return OperationResult{}, fmt.Errorf("unsupported device: Paper Agent currently supports Paper Pro Move (chiappa) only")
	}
	if !before.PaperAgent {
		return OperationResult{Status: before, Summary: "Paper Agent is not installed."}, nil
	}
	if _, err := maintenance.Uninstall(connection); err != nil {
		return OperationResult{}, err
	}
	after, err := preflight.Inspect(connection, host)
	if err != nil {
		return OperationResult{}, fmt.Errorf("verify uninstall: %w", err)
	}
	if after.PaperAgent || after.ServiceActive || after.SettingsApp {
		return OperationResult{}, fmt.Errorf("uninstall verification failed: Paper Agent components remain")
	}
	return OperationResult{
		Status:  after,
		Summary: "Paper Agent was removed. ChatGPT sign-in, XOVI/AppLoad, settings, runtime, and backups were preserved.",
	}, nil
}

func (a *App) ImplementationState() map[string]bool {
	return map[string]bool{
		"discovery": true,
		"preflight": true,
		"install":   false,
		"update":    false,
		"repair":    false,
		"login":     false,
		"uninstall": true,
	}
}
