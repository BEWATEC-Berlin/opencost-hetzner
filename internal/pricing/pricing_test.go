package pricing

import "testing"

func TestGenerateCustomPricingUsesWeightedBlendedNodeRates(t *testing.T) {
	input := Input{
		CurrencyMode: "net",
		Servers: []Server{
			{
				ID:            1,
				Name:          "worker-a",
				Type:          "cpx21",
				Location:      "fsn1",
				VCPU:          3,
				RAMGiB:        4,
				HourlyCostNet: 0.012,
			},
			{
				ID:            2,
				Name:          "worker-b",
				Type:          "cpx31",
				Location:      "fsn1",
				VCPU:          4,
				RAMGiB:        8,
				HourlyCostNet: 0.024,
			},
		},
		Volume: VolumePricing{PerGBMonthNet: 0.0476},
	}

	got, err := GenerateCustomPricing(input)
	if err != nil {
		t.Fatalf("GenerateCustomPricing() error = %v", err)
	}

	if got.Provider != "custom" {
		t.Fatalf("Provider = %q, want custom", got.Provider)
	}
	if got.CPU != "0.0025714286" {
		t.Fatalf("CPU = %q, want 0.0025714286", got.CPU)
	}
	if got.RAM != "0.0015" {
		t.Fatalf("RAM = %q, want 0.0015", got.RAM)
	}
	if got.Storage != "0.0000652055" {
		t.Fatalf("Storage = %q, want 0.0000652055", got.Storage)
	}
}

func TestGenerateCustomPricingRejectsNoServers(t *testing.T) {
	_, err := GenerateCustomPricing(Input{CurrencyMode: "net"})
	if err == nil {
		t.Fatal("GenerateCustomPricing() error = nil, want error")
	}
}

func TestGenerateCustomPricingRejectsInvalidCurrencyMode(t *testing.T) {
	_, err := GenerateCustomPricing(Input{
		CurrencyMode: "discounted",
		Servers: []Server{{
			ID:            1,
			Name:          "worker-a",
			Type:          "cpx21",
			Location:      "fsn1",
			VCPU:          3,
			RAMGiB:        4,
			HourlyCostNet: 0.012,
		}},
	})
	if err == nil {
		t.Fatal("GenerateCustomPricing() error = nil, want error")
	}
}

func TestGenerateCustomPricingRejectsMissingNetVolumePricing(t *testing.T) {
	_, err := GenerateCustomPricing(Input{
		CurrencyMode: "net",
		Servers: []Server{{
			ID:            1,
			Name:          "worker-a",
			Type:          "cpx21",
			Location:      "fsn1",
			VCPU:          3,
			RAMGiB:        4,
			HourlyCostNet: 0.012,
		}},
	})
	if err == nil {
		t.Fatal("GenerateCustomPricing() error = nil, want error")
	}
}

func TestGenerateCustomPricingRejectsMissingGrossVolumePricing(t *testing.T) {
	_, err := GenerateCustomPricing(Input{
		CurrencyMode: "gross",
		Servers: []Server{{
			ID:              1,
			Name:            "worker-a",
			Type:            "cpx21",
			Location:        "fsn1",
			VCPU:            3,
			RAMGiB:          4,
			HourlyCostGross: 0.0143,
		}},
		Volume: VolumePricing{PerGBMonthNet: 0.0476},
	})
	if err == nil {
		t.Fatal("GenerateCustomPricing() error = nil, want error")
	}
}
