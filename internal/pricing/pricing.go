package pricing

import (
	"fmt"
	"strconv"
	"strings"
)

const hoursPerMonth = 730

type Server struct {
	ProjectName     string
	ID              int64
	Name            string
	Type            string
	Location        string
	VCPU            int64
	RAMGiB          float64
	HourlyCostNet   float64
	HourlyCostGross float64
}

type VolumePricing struct {
	PerGBMonthNet   float64
	PerGBMonthGross float64
}

type Volume struct {
	ProjectName     string
	ID              int64
	Name            string
	Location        string
	SizeGB          int
	HourlyCostNet   float64
	HourlyCostGross float64
	Deleted         bool
}

type LoadBalancer struct {
	ProjectName     string
	ID              int64
	Name            string
	Type            string
	Location        string
	HourlyCostNet   float64
	HourlyCostGross float64
	Deleted         bool
}

type PrimaryIP struct {
	ProjectName     string
	ID              int64
	Name            string
	Type            string
	Location        string
	HourlyCostNet   float64
	HourlyCostGross float64
	Deleted         bool
}

type FloatingIP struct {
	ProjectName     string
	ID              int64
	Name            string
	Type            string
	Location        string
	HourlyCostNet   float64
	HourlyCostGross float64
	Deleted         bool
}

type TrafficObservation struct {
	ProjectName          string
	ResourceType         string
	ID                   int64
	Name                 string
	Location             string
	OutgoingBytes        uint64
	IncludedTrafficBytes uint64
	PerTBTrafficNet      float64
	PerTBTrafficGross    float64
}

type TrafficCost struct {
	ProjectName      string
	ResourceType     string
	ID               int64
	Name             string
	Location         string
	UsageTB          float64
	CostNet          float64
	CostGross        float64
	SourceResource   string
	SourceProviderID string
}

type Input struct {
	CurrencyMode        string
	Servers             []Server
	Volume              VolumePricing
	Volumes             []Volume
	LoadBalancers       []LoadBalancer
	PrimaryIPs          []PrimaryIP
	FloatingIPs         []FloatingIP
	TrafficObservations []TrafficObservation
	TrafficCosts        []TrafficCost
}

type CustomPricing struct {
	Provider    string `json:"provider"`
	Description string `json:"description"`
	CPU         string `json:"CPU"`
	RAM         string `json:"RAM"`
	Storage     string `json:"storage"`
}

func GenerateCustomPricing(input Input) (*CustomPricing, error) {
	if len(input.Servers) == 0 {
		return nil, fmt.Errorf("at least one server is required")
	}
	if input.CurrencyMode == "" {
		input.CurrencyMode = "net"
	}
	if input.CurrencyMode != "net" && input.CurrencyMode != "gross" {
		return nil, fmt.Errorf("currency mode must be net or gross")
	}

	var totalCPU int64
	var totalRAMGiB float64
	var totalHourlyCost float64
	for _, server := range input.Servers {
		if server.VCPU <= 0 {
			return nil, fmt.Errorf("server %q has invalid vcpu count %d", server.Name, server.VCPU)
		}
		if server.RAMGiB <= 0 {
			return nil, fmt.Errorf("server %q has invalid ram %.4f", server.Name, server.RAMGiB)
		}

		cost := server.HourlyCostNet
		if input.CurrencyMode == "gross" {
			cost = server.HourlyCostGross
		}
		if cost <= 0 {
			return nil, fmt.Errorf("server %q has invalid hourly cost %.10f", server.Name, cost)
		}

		totalCPU += server.VCPU
		totalRAMGiB += server.RAMGiB
		totalHourlyCost += cost
	}

	cpuPool := totalHourlyCost / 2
	ramPool := totalHourlyCost / 2
	storageMonthly := input.Volume.PerGBMonthNet
	if input.CurrencyMode == "gross" {
		storageMonthly = input.Volume.PerGBMonthGross
	}
	if storageMonthly <= 0 {
		return nil, fmt.Errorf("volume pricing must be positive, got %.10f", storageMonthly)
	}

	return &CustomPricing{
		Provider:    "custom",
		Description: "Generated Hetzner pricing bridge",
		CPU:         formatPrice(cpuPool / float64(totalCPU)),
		RAM:         formatPrice(ramPool / totalRAMGiB),
		Storage:     formatPrice(storageMonthly / hoursPerMonth),
	}, nil
}

func formatPrice(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 10, 64)
	formatted = strings.TrimRight(formatted, "0")
	return strings.TrimRight(formatted, ".")
}
