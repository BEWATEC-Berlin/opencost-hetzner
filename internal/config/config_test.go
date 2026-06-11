package config

import (
	"strings"
	"testing"
)

func TestLoadDefaultsCurrencyModeToNet(t *testing.T) {
	cfg, err := Load(strings.NewReader(`{
		"projects": [
			{"name": "prod", "token": "secret"}
		],
		"persistence": {"enabled": true, "path": "/data"},
		"kubernetes": {"storage_classes": ["hcloud-volumes"]},
		"include": {"servers": true}
	}`))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.CurrencyMode != "net" {
		t.Fatalf("CurrencyMode = %q, want net", cfg.CurrencyMode)
	}
	if !cfg.Include.Servers {
		t.Fatal("Include.Servers = false, want true")
	}
	if got := cfg.Kubernetes.StorageClasses[0]; got != "hcloud-volumes" {
		t.Fatalf("StorageClasses[0] = %q, want hcloud-volumes", got)
	}
}

func TestLoadRejectsMissingProjects(t *testing.T) {
	_, err := Load(strings.NewReader(`{"currency_mode":"net"}`))
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadRejectsDuplicateProjectNames(t *testing.T) {
	_, err := Load(strings.NewReader(`{
		"projects": [
			{"name": "prod", "token": "a"},
			{"name": "prod", "token": "b"}
		]
	}`))
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
