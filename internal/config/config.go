package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Config struct {
	LogLevel     string      `json:"log_level"`
	CurrencyMode string      `json:"currency_mode"`
	Projects     []Project   `json:"projects"`
	Persistence  Persistence `json:"persistence"`
	Include      Include     `json:"include"`
	Kubernetes   Kubernetes  `json:"kubernetes"`
}

type Project struct {
	Name          string `json:"name"`
	Token         string `json:"token"`
	LabelSelector string `json:"label_selector"`
}

type Persistence struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type Include struct {
	Servers       bool `json:"servers"`
	Volumes       bool `json:"volumes"`
	LoadBalancers bool `json:"load_balancers"`
	PrimaryIPs    bool `json:"primary_ips"`
	FloatingIPs   bool `json:"floating_ips"`
	Traffic       bool `json:"traffic"`
}

type Kubernetes struct {
	StorageClasses []string `json:"storage_classes"`
}

func Load(r io.Reader) (*Config, error) {
	var cfg Config
	if err := json.NewDecoder(r).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if cfg.CurrencyMode == "" {
		cfg.CurrencyMode = "net"
	}
	switch cfg.CurrencyMode {
	case "net", "gross":
	default:
		return nil, fmt.Errorf("currency_mode must be net or gross")
	}

	if len(cfg.Projects) == 0 {
		return nil, fmt.Errorf("at least one project is required")
	}

	seen := map[string]struct{}{}
	for idx, project := range cfg.Projects {
		project.Name = strings.TrimSpace(project.Name)
		project.Token = strings.TrimSpace(project.Token)
		if project.Name == "" {
			return nil, fmt.Errorf("projects[%d].name is required", idx)
		}
		if project.Token == "" {
			return nil, fmt.Errorf("projects[%d].token is required", idx)
		}
		if _, ok := seen[project.Name]; ok {
			return nil, fmt.Errorf("duplicate project name %q", project.Name)
		}
		seen[project.Name] = struct{}{}
		cfg.Projects[idx].Name = project.Name
		cfg.Projects[idx].Token = project.Token
	}

	if cfg.Persistence.Enabled {
		cfg.Persistence.Path = strings.TrimSpace(cfg.Persistence.Path)
		if cfg.Persistence.Path == "" {
			return nil, fmt.Errorf("persistence.path is required when persistence is enabled")
		}
	}

	return &cfg, nil
}
