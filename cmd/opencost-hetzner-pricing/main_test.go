package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bewatec/opencost-hetzner/internal/config"
	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

type fakeFetcher struct {
	input pricing.Input
	err   error
}

func (f fakeFetcher) FetchPricingInput(context.Context, *config.Config) (pricing.Input, error) {
	return f.input, f.err
}

func TestWriteHelmValues(t *testing.T) {
	var out bytes.Buffer
	err := writeHelmValues(&out, &pricing.CustomPricing{
		Description: "Generated Hetzner pricing bridge",
		CPU:         "0.001",
		RAM:         "0.002",
		Storage:     "0.0001",
	})
	if err != nil {
		t.Fatalf("writeHelmValues() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"opencost:",
		"provider: custom",
		`CPU: "0.001"`,
		`RAM: "0.002"`,
		`storage: "0.0001"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("helm output missing %q:\n%s", want, got)
		}
	}
}

func TestWriteHelmValuesReturnsWriteError(t *testing.T) {
	err := writeHelmValues(failingWriter{}, &pricing.CustomPricing{
		Description: "Generated Hetzner pricing bridge",
		CPU:         "0.001",
		RAM:         "0.002",
		Storage:     "0.0001",
	})
	if err == nil {
		t.Fatal("writeHelmValues() error = nil, want error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRunRejectsUnsupportedFormatBeforeReadingConfig(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), "/does/not/exist", "xml", &out)
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("run() error = %v, want unsupported format", err)
	}
}

func TestRunReturnsConfigOpenError(t *testing.T) {
	var out bytes.Buffer
	err := run(context.Background(), "/does/not/exist", "json", &out)
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "open config") {
		t.Fatalf("run() error = %v, want open config", err)
	}
}

func TestRunWithFetcherReturnsConfigValidationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hetzner_config.json")
	err := os.WriteFile(path, []byte(`{"currency_mode":"net"}`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out bytes.Buffer
	err = runWithFetcher(context.Background(), path, "json", &out, fakeFetcher{
		input: testPricingInput(),
	})
	if err == nil {
		t.Fatal("runWithFetcher() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("runWithFetcher() error = %v, want project validation error", err)
	}
}

func TestRunWithFetcherWritesJSON(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	err := runWithFetcher(context.Background(), configPath, "json", &out, fakeFetcher{
		input: testPricingInput(),
	})
	if err != nil {
		t.Fatalf("runWithFetcher() error = %v", err)
	}
	if !strings.Contains(out.String(), `"provider": "custom"`) {
		t.Fatalf("json output missing provider: %s", out.String())
	}
}

func TestRunWithFetcherWritesHelmValues(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	err := runWithFetcher(context.Background(), configPath, "helm-values", &out, fakeFetcher{
		input: testPricingInput(),
	})
	if err != nil {
		t.Fatalf("runWithFetcher() error = %v", err)
	}
	if !strings.Contains(out.String(), "opencost:") {
		t.Fatalf("helm output missing opencost root: %s", out.String())
	}
}

func TestRunWithFetcherWritesCSV(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	err := runWithFetcher(context.Background(), configPath, "csv", &out, fakeFetcher{
		input: testPricingInput(),
	})
	if err != nil {
		t.Fatalf("runWithFetcher() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"EndTimestamp,InstanceID,Region,AssetClass,InstanceIDField,InstanceType,MarketPriceHourly,Version",
		"hcloud://1,fsn1,node,spec.providerID,cpx21,0.012,v1",
		"hcloud-volumes,,pv,spec.storageClassName,,0.0000652055,v1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("csv output missing %q:\n%s", want, got)
		}
	}
}

func TestRunWithFetcherWritesCustomCosts(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	input := testPricingInput()
	input.Volumes = []pricing.Volume{{
		ID:            10,
		Name:          "data",
		Location:      "fsn1",
		SizeGB:        100,
		HourlyCostNet: 0.0065,
	}}

	err := runWithFetcher(context.Background(), configPath, "custom-costs", &out, fakeFetcher{
		input: input,
	})
	if err != nil {
		t.Fatalf("runWithFetcher() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		`"domain": "hetzner"`,
		`"resource_type": "volume"`,
		`"provider_id": "10"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("custom costs output missing %q:\n%s", want, got)
		}
	}
}

func TestRunWithFetcherWritesCustomCostsWithPersistence(t *testing.T) {
	configPath := writeTempConfigWithPersistence(t)
	var out bytes.Buffer
	input := testPricingInput()
	input.Volumes = []pricing.Volume{{
		ID:            10,
		Name:          "data",
		Location:      "fsn1",
		SizeGB:        100,
		HourlyCostNet: 0.0065,
	}}

	err := runWithFetcher(context.Background(), configPath, "custom-costs", &out, fakeFetcher{
		input: input,
	})
	if err != nil {
		t.Fatalf("runWithFetcher() error = %v", err)
	}
	if !strings.Contains(out.String(), `"resource_type": "volume"`) {
		t.Fatalf("custom costs output missing volume: %s", out.String())
	}
}

func TestRunWithFetcherReturnsFetcherError(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	err := runWithFetcher(context.Background(), configPath, "json", &out, fakeFetcher{
		err: errors.New("fetch failed"),
	})
	if err == nil {
		t.Fatal("runWithFetcher() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "fetch failed") {
		t.Fatalf("runWithFetcher() error = %v, want fetch failed", err)
	}
}

func TestRunWithFetcherReturnsPricingError(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	err := runWithFetcher(context.Background(), configPath, "json", &out, fakeFetcher{
		input: pricing.Input{CurrencyMode: "net"},
	})
	if err == nil {
		t.Fatal("runWithFetcher() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "server") {
		t.Fatalf("runWithFetcher() error = %v, want server error", err)
	}
}

func TestRunWithFetcherReturnsCSVGenerationError(t *testing.T) {
	configPath := writeTempConfig(t)
	var out bytes.Buffer

	err := runWithFetcher(context.Background(), configPath, "csv", &out, fakeFetcher{
		input: pricing.Input{
			CurrencyMode: "discounted",
			Servers: []pricing.Server{{
				ID:            1,
				Name:          "worker",
				Type:          "cpx21",
				Location:      "fsn1",
				VCPU:          3,
				RAMGiB:        4,
				HourlyCostNet: 0.012,
			}},
		},
	})
	if err == nil {
		t.Fatal("runWithFetcher() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "currency") {
		t.Fatalf("runWithFetcher() error = %v, want currency error", err)
	}
}

func TestIsSupportedFormatAcceptsCSV(t *testing.T) {
	if !isSupportedFormat("csv") {
		t.Fatal("isSupportedFormat(csv) = false, want true")
	}
	if !isSupportedFormat("custom-costs") {
		t.Fatal("isSupportedFormat(custom-costs) = false, want true")
	}
	if isSupportedFormat("xml") {
		t.Fatal("isSupportedFormat(xml) = true, want false")
	}
}

func TestDefaultWindowReturnsPreviousHour(t *testing.T) {
	start, end := defaultWindow(time.Date(2026, 6, 11, 12, 45, 0, 0, time.UTC))
	if start != time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC) {
		t.Fatalf("start = %s", start)
	}
	if end != time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) {
		t.Fatalf("end = %s", end)
	}
}

func TestApplyPersistenceComputesTrafficCostsFromPreviousSnapshot(t *testing.T) {
	dir := t.TempDir()
	tb := uint64(1_000_000_000_000)
	previous := testPricingInput()
	previous.TrafficObservations = []pricing.TrafficObservation{{
		ResourceType:         "server",
		ID:                   1,
		Name:                 "worker",
		Location:             "fsn1",
		OutgoingBytes:        2 * tb,
		IncludedTrafficBytes: tb,
		PerTBTrafficNet:      1,
	}}
	if _, err := applyPersistence(dir, previous, time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("applyPersistence(previous) error = %v", err)
	}

	current := testPricingInput()
	current.TrafficObservations = []pricing.TrafficObservation{{
		ResourceType:         "server",
		ID:                   1,
		Name:                 "worker",
		Location:             "fsn1",
		OutgoingBytes:        3 * tb,
		IncludedTrafficBytes: tb,
		PerTBTrafficNet:      1,
	}}
	got, err := applyPersistence(dir, current, time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("applyPersistence(current) error = %v", err)
	}
	if len(got.TrafficCosts) != 1 {
		t.Fatalf("len(TrafficCosts) = %d, want 1", len(got.TrafficCosts))
	}
}

func TestApplyPersistenceRestoresDeletedResourcesForOutput(t *testing.T) {
	dir := t.TempDir()
	previous := testPricingInput()
	previous.Volumes = []pricing.Volume{{
		ID:            10,
		Name:          "deleted-volume",
		Location:      "fsn1",
		SizeGB:        10,
		HourlyCostNet: 0.001,
	}}
	if _, err := applyPersistence(dir, previous, time.Date(2026, 6, 11, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("applyPersistence(previous) error = %v", err)
	}

	current := testPricingInput()
	got, err := applyPersistence(dir, current, time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("applyPersistence(current) error = %v", err)
	}
	if len(got.Volumes) != 1 {
		t.Fatalf("len(Volumes) = %d, want 1", len(got.Volumes))
	}
	if !got.Volumes[0].Deleted {
		t.Fatal("restored volume Deleted = false, want true")
	}
}

func writeTempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hetzner_config.json")
	err := os.WriteFile(path, []byte(`{
		"projects": [{"name": "prod", "token": "secret"}],
		"persistence": {"enabled": false},
		"kubernetes": {"storage_classes": ["hcloud-volumes"]}
	}`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeTempConfigWithPersistence(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hetzner_config.json")
	err := os.WriteFile(path, []byte(`{
		"projects": [{"name": "prod", "token": "secret"}],
		"persistence": {"enabled": true, "path": "`+filepath.Join(dir, "snapshots")+`"},
		"kubernetes": {"storage_classes": ["hcloud-volumes"]}
	}`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func testPricingInput() pricing.Input {
	return pricing.Input{
		CurrencyMode: "net",
		Servers: []pricing.Server{
			{
				ID:            1,
				Name:          "worker",
				Type:          "cpx21",
				Location:      "fsn1",
				VCPU:          3,
				RAMGiB:        4,
				HourlyCostNet: 0.012,
			},
		},
		Volume: pricing.VolumePricing{PerGBMonthNet: 0.0476},
	}
}

func TestMarshalJSON(t *testing.T) {
	got, err := marshalJSON(&pricing.CustomPricing{
		Provider: "custom",
		CPU:      "0.001",
	})
	if err != nil {
		t.Fatalf("marshalJSON() error = %v", err)
	}
	if !strings.Contains(string(got), `"provider": "custom"`) {
		t.Fatalf("json output missing provider: %s", got)
	}
}
