package hetzner

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bewatec/opencost-hetzner/internal/config"
	bridgepricing "github.com/bewatec/opencost-hetzner/internal/pricing"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type Fetcher struct {
	ApplicationName    string
	ApplicationVersion string
	NewClient          func(token string) projectClient
}

type projectClient interface {
	Pricing(ctx context.Context) (hcloud.Pricing, error)
	Servers(ctx context.Context, labelSelector string) ([]*hcloud.Server, error)
	Volumes(ctx context.Context, labelSelector string) ([]*hcloud.Volume, error)
	LoadBalancers(ctx context.Context, labelSelector string) ([]*hcloud.LoadBalancer, error)
	PrimaryIPs(ctx context.Context, labelSelector string) ([]*hcloud.PrimaryIP, error)
	FloatingIPs(ctx context.Context, labelSelector string) ([]*hcloud.FloatingIP, error)
}

type liveProjectClient struct {
	client *hcloud.Client
}

func (f Fetcher) FetchPricingInput(ctx context.Context, cfg *config.Config) (bridgepricing.Input, error) {
	input := bridgepricing.Input{
		CurrencyMode: cfg.CurrencyMode,
	}
	include := effectiveInclude(cfg.Include)

	for _, project := range cfg.Projects {
		projectInput, err := f.fetchProject(ctx, project, include)
		if err != nil {
			return bridgepricing.Input{}, fmt.Errorf("fetch project %q: %w", project.Name, err)
		}
		input.Servers = append(input.Servers, projectInput.Servers...)
		input.Volumes = append(input.Volumes, projectInput.Volumes...)
		input.LoadBalancers = append(input.LoadBalancers, projectInput.LoadBalancers...)
		input.PrimaryIPs = append(input.PrimaryIPs, projectInput.PrimaryIPs...)
		input.FloatingIPs = append(input.FloatingIPs, projectInput.FloatingIPs...)
		input.TrafficObservations = append(input.TrafficObservations, projectInput.TrafficObservations...)
		if input.Volume.PerGBMonthNet == 0 && projectInput.Volume.PerGBMonthNet != 0 {
			input.Volume = projectInput.Volume
		}
	}

	return input, nil
}

func (f Fetcher) fetchProject(ctx context.Context, project config.Project, include config.Include) (bridgepricing.Input, error) {
	client := f.newClient(project.Token)
	hpricing, err := client.Pricing(ctx)
	if err != nil {
		return bridgepricing.Input{}, fmt.Errorf("get pricing: %w", err)
	}

	input := bridgepricing.Input{
		Volume: bridgepricing.VolumePricing{
			PerGBMonthNet:   parsePriceOrZero(hpricing.Volume.PerGBMonthly.Net),
			PerGBMonthGross: parsePriceOrZero(hpricing.Volume.PerGBMonthly.Gross),
		},
	}

	if include.Servers {
		servers, err := client.Servers(ctx, project.LabelSelector)
		if err != nil {
			return bridgepricing.Input{}, fmt.Errorf("list servers: %w", err)
		}
		input.Servers = make([]bridgepricing.Server, 0, len(servers))
		for _, server := range servers {
			if server == nil || server.ServerType == nil || server.Location == nil {
				continue
			}
			hourlyNet, hourlyGross, err := serverTypeHourlyPrice(hpricing, server.ServerType.Name, server.Location.Name)
			if err != nil {
				return bridgepricing.Input{}, fmt.Errorf("server %q: %w", server.Name, err)
			}
			input.Servers = append(input.Servers, bridgepricing.Server{
				ProjectName:     project.Name,
				ID:              server.ID,
				Name:            server.Name,
				Type:            server.ServerType.Name,
				Location:        server.Location.Name,
				VCPU:            int64(server.ServerType.Cores),
				RAMGiB:          float64(server.ServerType.Memory),
				HourlyCostNet:   hourlyNet,
				HourlyCostGross: hourlyGross,
			})
			if include.Traffic {
				input.TrafficObservations = append(input.TrafficObservations, bridgepricing.TrafficObservation{
					ProjectName:          project.Name,
					ResourceType:         "server",
					ID:                   server.ID,
					Name:                 server.Name,
					Location:             server.Location.Name,
					OutgoingBytes:        server.OutgoingTraffic,
					IncludedTrafficBytes: server.IncludedTraffic,
					PerTBTrafficNet:      serverPerTBTrafficPrice(hpricing, server.ServerType.Name, server.Location.Name, "net"),
					PerTBTrafficGross:    serverPerTBTrafficPrice(hpricing, server.ServerType.Name, server.Location.Name, "gross"),
				})
			}
		}
	}

	if include.Volumes {
		volumes, err := client.Volumes(ctx, project.LabelSelector)
		if err != nil {
			return bridgepricing.Input{}, fmt.Errorf("list volumes: %w", err)
		}
		for _, volume := range volumes {
			if volume == nil || volume.Location == nil {
				continue
			}
			input.Volumes = append(input.Volumes, bridgepricing.Volume{
				ProjectName:     project.Name,
				ID:              volume.ID,
				Name:            volume.Name,
				Location:        volume.Location.Name,
				SizeGB:          volume.Size,
				HourlyCostNet:   input.Volume.PerGBMonthNet * float64(volume.Size) / 730,
				HourlyCostGross: input.Volume.PerGBMonthGross * float64(volume.Size) / 730,
			})
		}
	}

	if include.LoadBalancers {
		loadBalancers, err := client.LoadBalancers(ctx, project.LabelSelector)
		if err != nil {
			return bridgepricing.Input{}, fmt.Errorf("list load balancers: %w", err)
		}
		for _, lb := range loadBalancers {
			if lb == nil || lb.Location == nil || lb.LoadBalancerType == nil {
				continue
			}
			hourlyNet, hourlyGross, err := loadBalancerHourlyPrice(hpricing, lb.LoadBalancerType.Name, lb.Location.Name)
			if err != nil {
				return bridgepricing.Input{}, fmt.Errorf("load balancer %q: %w", lb.Name, err)
			}
			input.LoadBalancers = append(input.LoadBalancers, bridgepricing.LoadBalancer{
				ProjectName:     project.Name,
				ID:              lb.ID,
				Name:            lb.Name,
				Type:            lb.LoadBalancerType.Name,
				Location:        lb.Location.Name,
				HourlyCostNet:   hourlyNet,
				HourlyCostGross: hourlyGross,
			})
			if include.Traffic {
				input.TrafficObservations = append(input.TrafficObservations, bridgepricing.TrafficObservation{
					ProjectName:          project.Name,
					ResourceType:         "load_balancer",
					ID:                   lb.ID,
					Name:                 lb.Name,
					Location:             lb.Location.Name,
					OutgoingBytes:        lb.OutgoingTraffic,
					IncludedTrafficBytes: lb.IncludedTraffic,
					PerTBTrafficNet:      loadBalancerPerTBTrafficPrice(hpricing, lb.LoadBalancerType.Name, lb.Location.Name, "net"),
					PerTBTrafficGross:    loadBalancerPerTBTrafficPrice(hpricing, lb.LoadBalancerType.Name, lb.Location.Name, "gross"),
				})
			}
		}
	}

	if include.PrimaryIPs {
		primaryIPs, err := client.PrimaryIPs(ctx, project.LabelSelector)
		if err != nil {
			return bridgepricing.Input{}, fmt.Errorf("list primary IPs: %w", err)
		}
		for _, ip := range primaryIPs {
			if ip == nil || ip.Location == nil {
				continue
			}
			hourlyNet, hourlyGross, err := primaryIPHourlyPrice(hpricing, string(ip.Type), ip.Location.Name)
			if err != nil {
				return bridgepricing.Input{}, fmt.Errorf("primary IP %q: %w", ip.Name, err)
			}
			input.PrimaryIPs = append(input.PrimaryIPs, bridgepricing.PrimaryIP{
				ProjectName:     project.Name,
				ID:              ip.ID,
				Name:            ip.Name,
				Type:            string(ip.Type),
				Location:        ip.Location.Name,
				HourlyCostNet:   hourlyNet,
				HourlyCostGross: hourlyGross,
			})
		}
	}

	if include.FloatingIPs {
		floatingIPs, err := client.FloatingIPs(ctx, project.LabelSelector)
		if err != nil {
			return bridgepricing.Input{}, fmt.Errorf("list floating IPs: %w", err)
		}
		for _, ip := range floatingIPs {
			if ip == nil || ip.HomeLocation == nil {
				continue
			}
			hourlyNet, hourlyGross, err := floatingIPHourlyPrice(hpricing, string(ip.Type), ip.HomeLocation.Name)
			if err != nil {
				return bridgepricing.Input{}, fmt.Errorf("floating IP %q: %w", ip.Name, err)
			}
			input.FloatingIPs = append(input.FloatingIPs, bridgepricing.FloatingIP{
				ProjectName:     project.Name,
				ID:              ip.ID,
				Name:            ip.Name,
				Type:            string(ip.Type),
				Location:        ip.HomeLocation.Name,
				HourlyCostNet:   hourlyNet,
				HourlyCostGross: hourlyGross,
			})
		}
	}

	return input, nil
}

func effectiveInclude(include config.Include) config.Include {
	if include.Servers || include.Volumes || include.LoadBalancers || include.PrimaryIPs || include.FloatingIPs || include.Traffic {
		return include
	}
	return config.Include{
		Servers:       true,
		Volumes:       true,
		LoadBalancers: true,
		PrimaryIPs:    true,
		FloatingIPs:   true,
		Traffic:       true,
	}
}

func (f Fetcher) newClient(token string) projectClient {
	if f.NewClient != nil {
		return f.NewClient(token)
	}
	return liveProjectClient{
		client: hcloud.NewClient(
			hcloud.WithToken(token),
			hcloud.WithApplication(f.applicationName(), f.applicationVersion()),
		),
	}
}

func (c liveProjectClient) Pricing(ctx context.Context) (hcloud.Pricing, error) {
	pricing, _, err := c.client.Pricing.Get(ctx)
	return pricing, err
}

func (c liveProjectClient) Servers(ctx context.Context, labelSelector string) ([]*hcloud.Server, error) {
	return c.client.Server.AllWithOpts(ctx, hcloud.ServerListOpts{ListOpts: hcloud.ListOpts{LabelSelector: labelSelector}})
}

func (c liveProjectClient) Volumes(ctx context.Context, labelSelector string) ([]*hcloud.Volume, error) {
	return c.client.Volume.AllWithOpts(ctx, hcloud.VolumeListOpts{ListOpts: hcloud.ListOpts{LabelSelector: labelSelector}})
}

func (c liveProjectClient) LoadBalancers(ctx context.Context, labelSelector string) ([]*hcloud.LoadBalancer, error) {
	return c.client.LoadBalancer.AllWithOpts(ctx, hcloud.LoadBalancerListOpts{ListOpts: hcloud.ListOpts{LabelSelector: labelSelector}})
}

func (c liveProjectClient) PrimaryIPs(ctx context.Context, labelSelector string) ([]*hcloud.PrimaryIP, error) {
	return c.client.PrimaryIP.AllWithOpts(ctx, hcloud.PrimaryIPListOpts{ListOpts: hcloud.ListOpts{LabelSelector: labelSelector}})
}

func (c liveProjectClient) FloatingIPs(ctx context.Context, labelSelector string) ([]*hcloud.FloatingIP, error) {
	return c.client.FloatingIP.AllWithOpts(ctx, hcloud.FloatingIPListOpts{ListOpts: hcloud.ListOpts{LabelSelector: labelSelector}})
}

func (f Fetcher) applicationName() string {
	if f.ApplicationName == "" {
		return "opencost-hetzner"
	}
	return f.ApplicationName
}

func (f Fetcher) applicationVersion() string {
	if f.ApplicationVersion == "" {
		return "dev"
	}
	return f.ApplicationVersion
}

func serverTypeHourlyPrice(pricing hcloud.Pricing, serverTypeName, locationName string) (float64, float64, error) {
	for _, serverType := range pricing.ServerTypes {
		if serverType.ServerType == nil || serverType.ServerType.Name != serverTypeName {
			continue
		}
		for _, locationPricing := range serverType.Pricings {
			if locationPricing.Location == nil || locationPricing.Location.Name != locationName {
				continue
			}
			net, err := parsePrice(locationPricing.Hourly.Net)
			if err != nil {
				return 0, 0, fmt.Errorf("parse net hourly price: %w", err)
			}
			gross, err := parsePrice(locationPricing.Hourly.Gross)
			if err != nil {
				return 0, 0, fmt.Errorf("parse gross hourly price: %w", err)
			}
			return net, gross, nil
		}
	}
	return 0, 0, fmt.Errorf("no hourly price for server type %q in location %q", serverTypeName, locationName)
}

func serverPerTBTrafficPrice(pricing hcloud.Pricing, serverTypeName, locationName, mode string) float64 {
	for _, serverType := range pricing.ServerTypes {
		if serverType.ServerType == nil || serverType.ServerType.Name != serverTypeName {
			continue
		}
		for _, locationPricing := range serverType.Pricings {
			if locationPricing.Location == nil || locationPricing.Location.Name != locationName {
				continue
			}
			if mode == "gross" {
				return parsePriceOrZero(locationPricing.PerTBTraffic.Gross)
			}
			return parsePriceOrZero(locationPricing.PerTBTraffic.Net)
		}
	}
	return 0
}

func loadBalancerHourlyPrice(pricing hcloud.Pricing, loadBalancerTypeName, locationName string) (float64, float64, error) {
	for _, lbType := range pricing.LoadBalancerTypes {
		if lbType.LoadBalancerType == nil || lbType.LoadBalancerType.Name != loadBalancerTypeName {
			continue
		}
		for _, locationPricing := range lbType.Pricings {
			if locationPricing.Location == nil || locationPricing.Location.Name != locationName {
				continue
			}
			net, err := parsePrice(locationPricing.Hourly.Net)
			if err != nil {
				return 0, 0, fmt.Errorf("parse net hourly price: %w", err)
			}
			gross, err := parsePrice(locationPricing.Hourly.Gross)
			if err != nil {
				return 0, 0, fmt.Errorf("parse gross hourly price: %w", err)
			}
			return net, gross, nil
		}
	}
	return 0, 0, fmt.Errorf("no hourly price for load balancer type %q in location %q", loadBalancerTypeName, locationName)
}

func loadBalancerPerTBTrafficPrice(pricing hcloud.Pricing, loadBalancerTypeName, locationName, mode string) float64 {
	for _, lbType := range pricing.LoadBalancerTypes {
		if lbType.LoadBalancerType == nil || lbType.LoadBalancerType.Name != loadBalancerTypeName {
			continue
		}
		for _, locationPricing := range lbType.Pricings {
			if locationPricing.Location == nil || locationPricing.Location.Name != locationName {
				continue
			}
			if mode == "gross" {
				return parsePriceOrZero(locationPricing.PerTBTraffic.Gross)
			}
			return parsePriceOrZero(locationPricing.PerTBTraffic.Net)
		}
	}
	return 0
}

func primaryIPHourlyPrice(pricing hcloud.Pricing, ipType, locationName string) (float64, float64, error) {
	for _, primaryIP := range pricing.PrimaryIPs {
		if primaryIP.Type != ipType {
			continue
		}
		for _, locationPricing := range primaryIP.Pricings {
			if locationPricing.Location != locationName {
				continue
			}
			net, err := parsePrice(locationPricing.Hourly.Net)
			if err != nil {
				return 0, 0, fmt.Errorf("parse net hourly price: %w", err)
			}
			gross, err := parsePrice(locationPricing.Hourly.Gross)
			if err != nil {
				return 0, 0, fmt.Errorf("parse gross hourly price: %w", err)
			}
			return net, gross, nil
		}
	}
	return 0, 0, fmt.Errorf("no hourly price for primary IP type %q in location %q", ipType, locationName)
}

func floatingIPHourlyPrice(pricing hcloud.Pricing, ipType, locationName string) (float64, float64, error) {
	for _, floatingIP := range pricing.FloatingIPs {
		if string(floatingIP.Type) != ipType {
			continue
		}
		for _, locationPricing := range floatingIP.Pricings {
			if locationPricing.Location == nil || locationPricing.Location.Name != locationName {
				continue
			}
			net, err := parsePrice(locationPricing.Monthly.Net)
			if err != nil {
				return 0, 0, fmt.Errorf("parse net monthly price: %w", err)
			}
			gross, err := parsePrice(locationPricing.Monthly.Gross)
			if err != nil {
				return 0, 0, fmt.Errorf("parse gross monthly price: %w", err)
			}
			return net / 730, gross / 730, nil
		}
	}
	return 0, 0, fmt.Errorf("no monthly price for floating IP type %q in location %q", ipType, locationName)
}

func parsePriceOrZero(raw string) float64 {
	parsed, err := parsePrice(raw)
	if err != nil {
		return 0
	}
	return parsed
}

func parsePrice(raw string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", raw, err)
	}
	return value, nil
}
