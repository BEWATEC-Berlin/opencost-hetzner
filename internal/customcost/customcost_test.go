package customcost

import (
	"testing"
	"time"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

func TestGenerateIncludesNonNodeResources(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	result, err := Generate(Input{
		WindowStart: start,
		WindowEnd:   end,
		Pricing: pricing.Input{
			CurrencyMode: "net",
			Volumes: []pricing.Volume{{
				ProjectName:   "prod",
				ID:            10,
				Name:          "data",
				Location:      "fsn1",
				SizeGB:        100,
				HourlyCostNet: 0.0065,
			}},
			LoadBalancers: []pricing.LoadBalancer{{
				ID:            20,
				Name:          "ingress",
				Type:          "lb11",
				Location:      "fsn1",
				HourlyCostNet: 0.0081,
			}},
			PrimaryIPs: []pricing.PrimaryIP{{
				ID:            30,
				Name:          "primary",
				Type:          "ipv4",
				Location:      "fsn1",
				HourlyCostNet: 0.0015,
			}},
			FloatingIPs: []pricing.FloatingIP{{
				ID:            40,
				Name:          "floating",
				Type:          "ipv6",
				Location:      "fsn1",
				HourlyCostNet: 0.0005,
			}},
			TrafficCosts: []pricing.TrafficCost{{
				ProjectName:      "prod",
				ResourceType:     "traffic",
				ID:               50,
				Name:             "node-a",
				Location:         "fsn1",
				UsageTB:          1.5,
				CostNet:          1.5,
				CostGross:        1.785,
				SourceResource:   "server",
				SourceProviderID: "50",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(result.Costs) != 5 {
		t.Fatalf("len(Costs) = %d, want 5", len(result.Costs))
	}
	assertCost(t, result.Costs[0], "volume", "hcloud:volume:10:2026-06-01T00:00:00Z", 0.013)
	assertCost(t, result.Costs[1], "load_balancer", "hcloud:load_balancer:20:2026-06-01T00:00:00Z", 0.0162)
	assertCost(t, result.Costs[2], "primary_ip", "hcloud:primary_ip:30:2026-06-01T00:00:00Z", 0.003)
	assertCost(t, result.Costs[3], "floating_ip", "hcloud:floating_ip:40:2026-06-01T00:00:00Z", 0.001)
	assertCost(t, result.Costs[4], "traffic", "hcloud:traffic:server:50:2026-06-01T00:00:00Z", 1.5)
	if result.Costs[0].AccountName != "prod" {
		t.Fatalf("volume AccountName = %q, want prod", result.Costs[0].AccountName)
	}
	if result.Costs[4].AccountName != "prod" {
		t.Fatalf("traffic AccountName = %q, want prod", result.Costs[4].AccountName)
	}
}

func TestGenerateRejectsInvalidWindow(t *testing.T) {
	now := time.Now()
	_, err := Generate(Input{WindowStart: now, WindowEnd: now})
	if err == nil {
		t.Fatal("Generate() error = nil, want error")
	}
}

func TestGenerateUsesGrossMode(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	result, err := Generate(Input{
		WindowStart: start,
		WindowEnd:   end,
		Pricing: pricing.Input{
			CurrencyMode: "gross",
			Volumes: []pricing.Volume{{
				ID:              10,
				Name:            "data",
				Location:        "fsn1",
				SizeGB:          100,
				HourlyCostNet:   0.0065,
				HourlyCostGross: 0.0077,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Costs[0].BilledCost != 0.0077 {
		t.Fatalf("BilledCost = %.4f, want 0.0077", result.Costs[0].BilledCost)
	}
}

func TestGenerateLabelsDeletedResources(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	result, err := Generate(Input{
		WindowStart: start,
		WindowEnd:   end,
		Pricing: pricing.Input{
			CurrencyMode: "net",
			Volumes: []pricing.Volume{{
				ID:            10,
				Name:          "data",
				Location:      "fsn1",
				SizeGB:        100,
				HourlyCostNet: 0.0065,
				Deleted:       true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Costs[0].Labels["lifecycle"] != "deleted" {
		t.Fatalf("lifecycle label = %q, want deleted", result.Costs[0].Labels["lifecycle"])
	}
}

func TestGenerateHandlesZeroTrafficUsage(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	result, err := Generate(Input{
		WindowStart: start,
		WindowEnd:   end,
		Pricing: pricing.Input{
			CurrencyMode: "net",
			TrafficCosts: []pricing.TrafficCost{{
				ID:             50,
				Name:           "node-a",
				Location:       "fsn1",
				CostNet:        1,
				SourceResource: "server",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Costs[0].ListUnitPrice != 0 {
		t.Fatalf("ListUnitPrice = %.4f, want 0", result.Costs[0].ListUnitPrice)
	}
}

func assertCost(t *testing.T, got Cost, resourceType, id string, cost float64) {
	t.Helper()
	if got.ResourceType != resourceType {
		t.Fatalf("ResourceType = %q, want %q", got.ResourceType, resourceType)
	}
	if got.ID != id {
		t.Fatalf("ID = %q, want %q", got.ID, id)
	}
	if got.BilledCost != cost {
		t.Fatalf("BilledCost = %.10f, want %.10f", got.BilledCost, cost)
	}
}
