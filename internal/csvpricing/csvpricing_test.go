package csvpricing

import (
	"encoding/csv"
	"strings"
	"testing"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

func TestGenerateRowsIncludesExactNodeRows(t *testing.T) {
	rows, err := GenerateRows(Input{
		CurrencyMode: "net",
		Servers: []pricing.Server{
			{
				ID:            123456,
				Type:          "cpx31",
				Location:      "fsn1",
				HourlyCostNet: 0.024,
			},
			{
				ID:            234567,
				Type:          "ccx23",
				Location:      "nbg1",
				HourlyCostNet: 0.089,
			},
		},
	})
	if err != nil {
		t.Fatalf("GenerateRows() error = %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	assertRow(t, rows[0], Row{
		InstanceID:        "hcloud://123456",
		Region:            "fsn1",
		AssetClass:        "node",
		InstanceIDField:   "spec.providerID",
		InstanceType:      "cpx31",
		MarketPriceHourly: "0.024",
		Version:           "v1",
	})
	assertRow(t, rows[1], Row{
		InstanceID:        "hcloud://234567",
		Region:            "nbg1",
		AssetClass:        "node",
		InstanceIDField:   "spec.providerID",
		InstanceType:      "ccx23",
		MarketPriceHourly: "0.089",
		Version:           "v1",
	})
}

func TestGenerateRowsIncludesPVStorageClassRows(t *testing.T) {
	rows, err := GenerateRows(Input{
		CurrencyMode:   "net",
		StorageClasses: []string{"hcloud-volumes", "fast"},
		Volume: pricing.VolumePricing{
			PerGBMonthNet: 0.0572,
		},
	})
	if err != nil {
		t.Fatalf("GenerateRows() error = %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	assertRow(t, rows[0], Row{
		InstanceID:        "hcloud-volumes",
		AssetClass:        "pv",
		InstanceIDField:   "spec.storageClassName",
		MarketPriceHourly: "0.0000783562",
		Version:           "v1",
	})
	assertRow(t, rows[1], Row{
		InstanceID:        "fast",
		AssetClass:        "pv",
		InstanceIDField:   "spec.storageClassName",
		MarketPriceHourly: "0.0000783562",
		Version:           "v1",
	})
}

func TestWriteCSVIncludesOpenCostHeader(t *testing.T) {
	var out strings.Builder
	err := Write(&out, []Row{{
		InstanceID:        "hcloud://123456",
		Region:            "fsn1",
		AssetClass:        "node",
		InstanceIDField:   "spec.providerID",
		InstanceType:      "cpx31",
		MarketPriceHourly: "0.024",
		Version:           "v1",
	}})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("read generated csv: %v", err)
	}
	if got, want := records[0][0], "EndTimestamp"; got != want {
		t.Fatalf("header[0] = %q, want %q", got, want)
	}
	if got, want := records[1][1], "hcloud://123456"; got != want {
		t.Fatalf("record InstanceID = %q, want %q", got, want)
	}
}

func TestGenerateRowsRejectsInvalidCurrencyMode(t *testing.T) {
	_, err := GenerateRows(Input{CurrencyMode: "discounted"})
	if err == nil {
		t.Fatal("GenerateRows() error = nil, want error")
	}
}

func TestGenerateRowsRejectsInvalidServerCost(t *testing.T) {
	_, err := GenerateRows(Input{
		CurrencyMode: "net",
		Servers: []pricing.Server{{
			ID:       123456,
			Name:     "worker",
			Type:     "cpx31",
			Location: "fsn1",
		}},
	})
	if err == nil {
		t.Fatal("GenerateRows() error = nil, want error")
	}
}

func TestGenerateRowsUsesGrossPrices(t *testing.T) {
	rows, err := GenerateRows(Input{
		CurrencyMode: "gross",
		Servers: []pricing.Server{{
			ID:              123456,
			Type:            "cpx31",
			Location:        "fsn1",
			HourlyCostNet:   0.024,
			HourlyCostGross: 0.02856,
		}},
		Volume: pricing.VolumePricing{PerGBMonthGross: 0.0566},
		StorageClasses: []string{
			"hcloud-volumes",
		},
	})
	if err != nil {
		t.Fatalf("GenerateRows() error = %v", err)
	}
	if rows[0].MarketPriceHourly != "0.02856" {
		t.Fatalf("node MarketPriceHourly = %q, want 0.02856", rows[0].MarketPriceHourly)
	}
	if rows[1].MarketPriceHourly != "0.0000775342" {
		t.Fatalf("pv MarketPriceHourly = %q, want 0.0000775342", rows[1].MarketPriceHourly)
	}
}

func assertRow(t *testing.T, got Row, want Row) {
	t.Helper()
	if got != want {
		t.Fatalf("row = %#v, want %#v", got, want)
	}
}
