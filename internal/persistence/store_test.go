package persistence

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

func TestStoreRoundTripLatestSnapshot(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	snapshot := Snapshot{
		CapturedAt: time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
		Pricing: pricing.Input{
			CurrencyMode: "net",
			Servers: []pricing.Server{{
				ID:            1,
				Name:          "node-a",
				HourlyCostNet: 0.01,
			}},
		},
	}

	if err := store.SaveLatest(snapshot); err != nil {
		t.Fatalf("SaveLatest() error = %v", err)
	}
	got, ok, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}
	if !ok {
		t.Fatal("LoadLatest() ok = false, want true")
	}
	if got.Pricing.Servers[0].ID != 1 {
		t.Fatalf("server ID = %d, want 1", got.Pricing.Servers[0].ID)
	}
}

func TestLoadLatestMissingSnapshot(t *testing.T) {
	_, ok, err := (Store{Dir: t.TempDir()}).LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}
	if ok {
		t.Fatal("LoadLatest() ok = true, want false")
	}
}

func TestLoadLatestRejectsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, latestFile), []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid snapshot: %v", err)
	}
	_, _, err := (Store{Dir: dir}).LoadLatest()
	if err == nil {
		t.Fatal("LoadLatest() error = nil, want error")
	}
}

func TestSaveLatestRejectsFileAsDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	err := (Store{Dir: path}).SaveLatest(Snapshot{CapturedAt: time.Now()})
	if err == nil {
		t.Fatal("SaveLatest() error = nil, want error")
	}
}
