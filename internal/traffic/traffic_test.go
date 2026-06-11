package traffic

import (
	"testing"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

func TestComputeCostsBillsOnlyNewOverage(t *testing.T) {
	tb := uint64(1_000_000_000_000)
	costs := ComputeCosts(
		pricing.Input{
			CurrencyMode: "net",
			TrafficObservations: []pricing.TrafficObservation{{
				ResourceType:         "server",
				ID:                   1,
				Name:                 "node-a",
				Location:             "fsn1",
				OutgoingBytes:        2 * tb,
				IncludedTrafficBytes: tb,
				PerTBTrafficNet:      1,
				PerTBTrafficGross:    1.19,
			}},
		},
		pricing.Input{
			CurrencyMode: "net",
			TrafficObservations: []pricing.TrafficObservation{{
				ResourceType:         "server",
				ID:                   1,
				Name:                 "node-a",
				Location:             "fsn1",
				OutgoingBytes:        3 * tb,
				IncludedTrafficBytes: tb,
				PerTBTrafficNet:      1,
				PerTBTrafficGross:    1.19,
			}},
		},
	)

	if len(costs) != 1 {
		t.Fatalf("len(costs) = %d, want 1", len(costs))
	}
	if costs[0].UsageTB != 1 {
		t.Fatalf("UsageTB = %.4f, want 1", costs[0].UsageTB)
	}
	if costs[0].CostNet != 1 {
		t.Fatalf("CostNet = %.4f, want 1", costs[0].CostNet)
	}
}

func TestComputeCostsIgnoresIncludedTraffic(t *testing.T) {
	costs := ComputeCosts(pricing.Input{}, pricing.Input{
		CurrencyMode: "net",
		TrafficObservations: []pricing.TrafficObservation{{
			ResourceType:         "server",
			ID:                   1,
			OutgoingBytes:        100,
			IncludedTrafficBytes: 200,
			PerTBTrafficNet:      1,
		}},
	})
	if len(costs) != 0 {
		t.Fatalf("len(costs) = %d, want 0", len(costs))
	}
}

func TestComputeCostsHandlesCounterReset(t *testing.T) {
	costs := ComputeCosts(
		pricing.Input{
			CurrencyMode: "net",
			TrafficObservations: []pricing.TrafficObservation{{
				ResourceType:         "load_balancer",
				ID:                   2,
				OutgoingBytes:        500,
				IncludedTrafficBytes: 100,
				PerTBTrafficNet:      1,
			}},
		},
		pricing.Input{
			CurrencyMode: "net",
			TrafficObservations: []pricing.TrafficObservation{{
				ResourceType:         "load_balancer",
				ID:                   2,
				OutgoingBytes:        200,
				IncludedTrafficBytes: 100,
				PerTBTrafficNet:      1,
			}},
		},
	)
	if len(costs) != 0 {
		t.Fatalf("len(costs) = %d, want 0", len(costs))
	}
}

func TestComputeCostsKeysByProjectAndResource(t *testing.T) {
	tb := uint64(1_000_000_000_000)
	costs := ComputeCosts(
		pricing.Input{
			TrafficObservations: []pricing.TrafficObservation{{
				ProjectName:          "prod",
				ResourceType:         "server",
				ID:                   1,
				OutgoingBytes:        2 * tb,
				IncludedTrafficBytes: tb,
				PerTBTrafficNet:      1,
			}},
		},
		pricing.Input{
			TrafficObservations: []pricing.TrafficObservation{{
				ProjectName:          "stage",
				ResourceType:         "server",
				ID:                   1,
				Name:                 "node-stage",
				OutgoingBytes:        2 * tb,
				IncludedTrafficBytes: tb,
				PerTBTrafficNet:      1,
			}},
		},
	)

	if len(costs) != 1 {
		t.Fatalf("len(costs) = %d, want 1", len(costs))
	}
	if costs[0].ProjectName != "stage" {
		t.Fatalf("ProjectName = %q, want stage", costs[0].ProjectName)
	}
}
