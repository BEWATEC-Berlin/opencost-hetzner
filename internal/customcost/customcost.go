package customcost

import (
	"fmt"
	"time"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

type Input struct {
	WindowStart time.Time
	WindowEnd   time.Time
	Pricing     pricing.Input
}

type Response struct {
	Domain     string    `json:"domain"`
	CostSource string    `json:"cost_source"`
	Version    string    `json:"version"`
	Currency   string    `json:"currency"`
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Costs      []Cost    `json:"costs"`
}

type Cost struct {
	ID             string            `json:"id"`
	AccountName    string            `json:"account_name,omitempty"`
	ProviderID     string            `json:"provider_id"`
	ResourceName   string            `json:"resource_name"`
	ResourceType   string            `json:"resource_type"`
	Zone           string            `json:"zone"`
	ChargeCategory string            `json:"charge_category"`
	Description    string            `json:"description"`
	BilledCost     float64           `json:"billed_cost"`
	ListCost       float64           `json:"list_cost"`
	ListUnitPrice  float64           `json:"list_unit_price"`
	UsageQuantity  float64           `json:"usage_quantity"`
	UsageUnit      string            `json:"usage_unit"`
	Labels         map[string]string `json:"labels,omitempty"`
}

func Generate(input Input) (*Response, error) {
	if !input.WindowEnd.After(input.WindowStart) {
		return nil, fmt.Errorf("window end must be after window start")
	}
	currency := "EUR"
	mode := input.Pricing.CurrencyMode
	if mode == "" {
		mode = "net"
	}
	if mode != "net" && mode != "gross" {
		return nil, fmt.Errorf("currency mode must be net or gross")
	}

	hours := input.WindowEnd.Sub(input.WindowStart).Hours()
	resp := &Response{
		Domain:     "hetzner",
		CostSource: "cloud infrastructure",
		Version:    "v1",
		Currency:   currency,
		Start:      input.WindowStart,
		End:        input.WindowEnd,
		Costs:      []Cost{},
	}

	for _, volume := range input.Pricing.Volumes {
		resp.Costs = append(resp.Costs, resourceCost(resourceCostInput{
			windowStart:  input.WindowStart,
			accountName:  volume.ProjectName,
			providerID:   fmt.Sprintf("%d", volume.ID),
			resourceName: volume.Name,
			resourceType: "volume",
			zone:         volume.Location,
			description:  fmt.Sprintf("Hetzner volume %s (%d GB)", volume.Name, volume.SizeGB),
			hourlyCost:   resourceHourlyCost(mode, volume.HourlyCostNet, volume.HourlyCostGross),
			hours:        hours,
			usageUnit:    "hour",
			deleted:      volume.Deleted,
		}))
	}
	for _, lb := range input.Pricing.LoadBalancers {
		resp.Costs = append(resp.Costs, resourceCost(resourceCostInput{
			windowStart:  input.WindowStart,
			accountName:  lb.ProjectName,
			providerID:   fmt.Sprintf("%d", lb.ID),
			resourceName: lb.Name,
			resourceType: "load_balancer",
			zone:         lb.Location,
			description:  fmt.Sprintf("Hetzner load balancer %s (%s)", lb.Name, lb.Type),
			hourlyCost:   resourceHourlyCost(mode, lb.HourlyCostNet, lb.HourlyCostGross),
			hours:        hours,
			usageUnit:    "hour",
			deleted:      lb.Deleted,
		}))
	}
	for _, ip := range input.Pricing.PrimaryIPs {
		resp.Costs = append(resp.Costs, resourceCost(resourceCostInput{
			windowStart:  input.WindowStart,
			accountName:  ip.ProjectName,
			providerID:   fmt.Sprintf("%d", ip.ID),
			resourceName: ip.Name,
			resourceType: "primary_ip",
			zone:         ip.Location,
			description:  fmt.Sprintf("Hetzner primary IP %s (%s)", ip.Name, ip.Type),
			hourlyCost:   resourceHourlyCost(mode, ip.HourlyCostNet, ip.HourlyCostGross),
			hours:        hours,
			usageUnit:    "hour",
			deleted:      ip.Deleted,
		}))
	}
	for _, ip := range input.Pricing.FloatingIPs {
		resp.Costs = append(resp.Costs, resourceCost(resourceCostInput{
			windowStart:  input.WindowStart,
			accountName:  ip.ProjectName,
			providerID:   fmt.Sprintf("%d", ip.ID),
			resourceName: ip.Name,
			resourceType: "floating_ip",
			zone:         ip.Location,
			description:  fmt.Sprintf("Hetzner floating IP %s (%s)", ip.Name, ip.Type),
			hourlyCost:   resourceHourlyCost(mode, ip.HourlyCostNet, ip.HourlyCostGross),
			hours:        hours,
			usageUnit:    "hour",
			deleted:      ip.Deleted,
		}))
	}
	for _, trafficCost := range input.Pricing.TrafficCosts {
		hourlyCost := resourceHourlyCost(mode, trafficCost.CostNet, trafficCost.CostGross)
		listUnitPrice := 0.0
		if trafficCost.UsageTB > 0 {
			listUnitPrice = hourlyCost / trafficCost.UsageTB
		}
		resp.Costs = append(resp.Costs, Cost{
			ID:             fmt.Sprintf("hcloud:traffic:%s:%d:%s", trafficCost.SourceResource, trafficCost.ID, input.WindowStart.Format(time.RFC3339)),
			AccountName:    trafficCost.ProjectName,
			ProviderID:     trafficCost.SourceProviderID,
			ResourceName:   trafficCost.Name,
			ResourceType:   "traffic",
			Zone:           trafficCost.Location,
			ChargeCategory: "usage",
			Description:    fmt.Sprintf("Hetzner traffic overage for %s %s", trafficCost.SourceResource, trafficCost.Name),
			BilledCost:     hourlyCost,
			ListCost:       hourlyCost,
			ListUnitPrice:  listUnitPrice,
			UsageQuantity:  trafficCost.UsageTB,
			UsageUnit:      "TB",
			Labels: map[string]string{
				"source_resource": trafficCost.SourceResource,
			},
		})
	}

	return resp, nil
}

type resourceCostInput struct {
	windowStart  time.Time
	accountName  string
	providerID   string
	resourceName string
	resourceType string
	zone         string
	description  string
	hourlyCost   float64
	hours        float64
	usageUnit    string
	deleted      bool
}

func resourceCost(input resourceCostInput) Cost {
	total := input.hourlyCost * input.hours
	labels := map[string]string{}
	if input.deleted {
		labels["lifecycle"] = "deleted"
		input.description = input.description + " (deleted since previous snapshot)"
	}
	return Cost{
		ID:             fmt.Sprintf("hcloud:%s:%s:%s", input.resourceType, input.providerID, input.windowStart.Format(time.RFC3339)),
		AccountName:    input.accountName,
		ProviderID:     input.providerID,
		ResourceName:   input.resourceName,
		ResourceType:   input.resourceType,
		Zone:           input.zone,
		ChargeCategory: "usage",
		Description:    input.description,
		BilledCost:     total,
		ListCost:       total,
		ListUnitPrice:  input.hourlyCost,
		UsageQuantity:  input.hours,
		UsageUnit:      input.usageUnit,
		Labels:         labels,
	}
}

func resourceHourlyCost(mode string, net, gross float64) float64 {
	if mode == "gross" {
		return gross
	}
	return net
}
