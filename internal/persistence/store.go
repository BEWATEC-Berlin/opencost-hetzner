package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

const latestFile = "latest.json"

type Store struct {
	Dir string
}

type Snapshot struct {
	CapturedAt time.Time     `json:"captured_at"`
	Pricing    pricing.Input `json:"pricing"`
}

func (s Store) LoadLatest() (Snapshot, bool, error) {
	path := filepath.Join(s.Dir, latestFile)
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, fmt.Errorf("open latest snapshot: %w", err)
	}
	defer file.Close()

	var snapshot Snapshot
	if err := json.NewDecoder(file).Decode(&snapshot); err != nil {
		return Snapshot{}, false, fmt.Errorf("decode latest snapshot: %w", err)
	}
	return snapshot, true, nil
}

func (s Store) SaveLatest(snapshot Snapshot) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}
	path := filepath.Join(s.Dir, latestFile)
	tempPath := path + ".tmp"

	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open temp snapshot: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(snapshot)
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode snapshot: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close snapshot: %w", closeErr)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace latest snapshot: %w", err)
	}
	return nil
}
