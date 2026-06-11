package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/bewatec/opencost-hetzner/internal/config"
	"github.com/bewatec/opencost-hetzner/internal/csvpricing"
	"github.com/bewatec/opencost-hetzner/internal/customcost"
	"github.com/bewatec/opencost-hetzner/internal/hetzner"
	"github.com/bewatec/opencost-hetzner/internal/lifecycle"
	"github.com/bewatec/opencost-hetzner/internal/persistence"
	"github.com/bewatec/opencost-hetzner/internal/pricing"
	"github.com/bewatec/opencost-hetzner/internal/traffic"
)

type pricingInputFetcher interface {
	FetchPricingInput(ctx context.Context, cfg *config.Config) (pricing.Input, error)
}

func main() {
	configPath := flag.String("config", "hetzner_config.json", "path to hetzner_config.json")
	format := flag.String("format", "json", "output format: json, helm-values, csv, or custom-costs")
	flag.Parse()

	if err := run(context.Background(), *configPath, *format, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "opencost-hetzner-pricing: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath, format string, stdout io.Writer) error {
	return runWithFetcher(ctx, configPath, format, stdout, hetzner.Fetcher{})
}

func runWithFetcher(ctx context.Context, configPath, format string, stdout io.Writer, fetcher pricingInputFetcher) error {
	if !isSupportedFormat(format) {
		return fmt.Errorf("unsupported format %q", format)
	}

	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	cfg, err := config.Load(file)
	if err != nil {
		return err
	}

	input, err := fetcher.FetchPricingInput(ctx, cfg)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		customPricing, err := pricing.GenerateCustomPricing(input)
		if err != nil {
			return err
		}
		output, err := marshalJSON(customPricing)
		if err != nil {
			return err
		}
		_, err = stdout.Write(output)
		return err
	case "helm-values":
		customPricing, err := pricing.GenerateCustomPricing(input)
		if err != nil {
			return err
		}
		return writeHelmValues(stdout, customPricing)
	case "csv":
		rows, err := csvpricing.GenerateRows(csvpricing.Input{
			CurrencyMode:   input.CurrencyMode,
			Servers:        input.Servers,
			Volume:         input.Volume,
			StorageClasses: cfg.Kubernetes.StorageClasses,
		})
		if err != nil {
			return err
		}
		return csvpricing.Write(stdout, rows)
	case "custom-costs":
		now := time.Now().UTC()
		if cfg.Persistence.Enabled {
			input, err = applyPersistence(cfg.Persistence.Path, input, now)
			if err != nil {
				return err
			}
		}
		windowStart, windowEnd := defaultWindow(now)
		response, err := customcost.Generate(customcost.Input{
			WindowStart: windowStart,
			WindowEnd:   windowEnd,
			Pricing:     input,
		})
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(response)
	}
	return fmt.Errorf("unsupported format %q", format)
}

func applyPersistence(path string, input pricing.Input, now time.Time) (pricing.Input, error) {
	store := persistence.Store{Dir: path}
	previous, ok, err := store.LoadLatest()
	if err != nil {
		return pricing.Input{}, err
	}
	output := input
	if ok {
		output = lifecycle.RestoreDeletedResources(previous.Pricing, input)
		output.TrafficCosts = traffic.ComputeCosts(previous.Pricing, input)
	}
	if err := store.SaveLatest(persistence.Snapshot{
		CapturedAt: now,
		Pricing:    input,
	}); err != nil {
		return pricing.Input{}, err
	}
	return output, nil
}

func isSupportedFormat(format string) bool {
	return format == "json" || format == "helm-values" || format == "csv" || format == "custom-costs"
}

func defaultWindow(now time.Time) (time.Time, time.Time) {
	end := now.Truncate(time.Hour)
	return end.Add(-time.Hour), end
}

func writeHelmValues(stdout io.Writer, customPricing *pricing.CustomPricing) error {
	var err error
	write := func(format string, args ...interface{}) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(stdout, format, args...)
	}

	write("opencost:\n")
	write("  customPricing:\n")
	write("    enabled: true\n")
	write("    provider: custom\n")
	write("    costModel:\n")
	write("      description: %q\n", customPricing.Description)
	write("      CPU: %q\n", customPricing.CPU)
	write("      RAM: %q\n", customPricing.RAM)
	write("      storage: %q\n", customPricing.Storage)
	return err
}

func marshalJSON(customPricing *pricing.CustomPricing) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(customPricing); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
