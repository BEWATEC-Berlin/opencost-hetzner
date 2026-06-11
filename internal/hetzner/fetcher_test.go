package hetzner

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/bewatec/opencost-hetzner/internal/config"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type fakeProjectClient struct {
	pricing       hcloud.Pricing
	servers       []*hcloud.Server
	volumes       []*hcloud.Volume
	loadBalancers []*hcloud.LoadBalancer
	primaryIPs    []*hcloud.PrimaryIP
	floatingIPs   []*hcloud.FloatingIP
	err           error
	serversErr    error
	volumesErr    error
	lbsErr        error
	primaryErr    error
	floatingErr   error
}

func (f fakeProjectClient) Pricing(context.Context) (hcloud.Pricing, error) {
	if f.err != nil {
		return hcloud.Pricing{}, f.err
	}
	return f.pricing, nil
}

func (f fakeProjectClient) Servers(context.Context, string) ([]*hcloud.Server, error) {
	if f.serversErr != nil {
		return nil, f.serversErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.servers, nil
}

func (f fakeProjectClient) Volumes(context.Context, string) ([]*hcloud.Volume, error) {
	if f.volumesErr != nil {
		return nil, f.volumesErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.volumes, nil
}

func (f fakeProjectClient) LoadBalancers(context.Context, string) ([]*hcloud.LoadBalancer, error) {
	if f.lbsErr != nil {
		return nil, f.lbsErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.loadBalancers, nil
}

func (f fakeProjectClient) PrimaryIPs(context.Context, string) ([]*hcloud.PrimaryIP, error) {
	if f.primaryErr != nil {
		return nil, f.primaryErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.primaryIPs, nil
}

func (f fakeProjectClient) FloatingIPs(context.Context, string) ([]*hcloud.FloatingIP, error) {
	if f.floatingErr != nil {
		return nil, f.floatingErr
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.floatingIPs, nil
}

func TestFetchPricingInputAggregatesProjects(t *testing.T) {
	fetcher := Fetcher{
		NewClient: func(token string) projectClient {
			return fakeProjectClient{
				pricing: testPricing(),
				servers: []*hcloud.Server{
					{
						ID:              100,
						Name:            "worker-" + token,
						Location:        &hcloud.Location{Name: "fsn1"},
						IncludedTraffic: 1_000_000_000_000,
						OutgoingTraffic: 2_000_000_000_000,
						ServerType: &hcloud.ServerType{
							Name:   "cpx21",
							Cores:  3,
							Memory: 4,
						},
					},
				},
			}
		},
	}

	input, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
		CurrencyMode: "net",
		Projects: []config.Project{
			{Name: "prod", Token: "prod-token"},
			{Name: "stage", Token: "stage-token"},
		},
	})
	if err != nil {
		t.Fatalf("FetchPricingInput() error = %v", err)
	}

	if len(input.Servers) != 2 {
		t.Fatalf("len(Servers) = %d, want 2", len(input.Servers))
	}
	if input.Servers[0].ProjectName != "prod" || input.Servers[1].ProjectName != "stage" {
		t.Fatalf("server project names = %q/%q, want prod/stage", input.Servers[0].ProjectName, input.Servers[1].ProjectName)
	}
	if input.Volume.PerGBMonthNet != 0.0476 {
		t.Fatalf("Volume.PerGBMonthNet = %.4f, want 0.0476", input.Volume.PerGBMonthNet)
	}
	if len(input.TrafficObservations) != 2 {
		t.Fatalf("len(TrafficObservations) = %d, want 2", len(input.TrafficObservations))
	}
	if input.TrafficObservations[0].PerTBTrafficNet != 1 {
		t.Fatalf("server PerTBTrafficNet = %.4f, want 1", input.TrafficObservations[0].PerTBTrafficNet)
	}
	if input.TrafficObservations[0].ProjectName != "prod" {
		t.Fatalf("traffic ProjectName = %q, want prod", input.TrafficObservations[0].ProjectName)
	}
}

func TestFetchPricingInputMapsNonNodeResources(t *testing.T) {
	fetcher := Fetcher{
		NewClient: func(string) projectClient {
			return fakeProjectClient{
				pricing: testPricing(),
				volumes: []*hcloud.Volume{{
					ID:       10,
					Name:     "data",
					Location: &hcloud.Location{Name: "fsn1"},
					Size:     100,
				}},
				loadBalancers: []*hcloud.LoadBalancer{{
					ID:               20,
					Name:             "ingress",
					Location:         &hcloud.Location{Name: "fsn1"},
					LoadBalancerType: &hcloud.LoadBalancerType{Name: "lb11"},
					IncludedTraffic:  1_000_000_000_000,
					OutgoingTraffic:  2_000_000_000_000,
				}},
				primaryIPs: []*hcloud.PrimaryIP{{
					ID:       30,
					Name:     "primary",
					Type:     hcloud.PrimaryIPTypeIPv4,
					Location: &hcloud.Location{Name: "fsn1"},
				}},
				floatingIPs: []*hcloud.FloatingIP{{
					ID:           40,
					Name:         "floating",
					Type:         hcloud.FloatingIPTypeIPv6,
					HomeLocation: &hcloud.Location{Name: "fsn1"},
				}},
			}
		},
	}

	input, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
		CurrencyMode: "net",
		Projects:     []config.Project{{Name: "prod", Token: "token"}},
	})
	if err != nil {
		t.Fatalf("FetchPricingInput() error = %v", err)
	}

	if len(input.Volumes) != 1 || !closeEnough(input.Volumes[0].HourlyCostNet, 0.00652054794520548) {
		t.Fatalf("Volumes = %#v, want one priced volume", input.Volumes)
	}
	if input.Volumes[0].ProjectName != "prod" {
		t.Fatalf("volume ProjectName = %q, want prod", input.Volumes[0].ProjectName)
	}
	if len(input.LoadBalancers) != 1 || input.LoadBalancers[0].HourlyCostNet != 0.0081 {
		t.Fatalf("LoadBalancers = %#v, want one priced load balancer", input.LoadBalancers)
	}
	if len(input.PrimaryIPs) != 1 || input.PrimaryIPs[0].HourlyCostNet != 0.0015 {
		t.Fatalf("PrimaryIPs = %#v, want one priced primary IP", input.PrimaryIPs)
	}
	if len(input.FloatingIPs) != 1 || input.FloatingIPs[0].HourlyCostNet != 0.0005 {
		t.Fatalf("FloatingIPs = %#v, want one priced floating IP", input.FloatingIPs)
	}
	if len(input.TrafficObservations) != 1 {
		t.Fatalf("len(TrafficObservations) = %d, want 1", len(input.TrafficObservations))
	}
	if input.TrafficObservations[0].PerTBTrafficNet != 2 {
		t.Fatalf("load balancer PerTBTrafficNet = %.4f, want 2", input.TrafficObservations[0].PerTBTrafficNet)
	}
}

func TestFetchPricingInputHonorsIncludeFlags(t *testing.T) {
	fetcher := Fetcher{
		NewClient: func(string) projectClient {
			return fakeProjectClient{
				pricing:     testPricing(),
				volumesErr:  errors.New("volumes should not be fetched"),
				lbsErr:      errors.New("load balancers should not be fetched"),
				primaryErr:  errors.New("primary IPs should not be fetched"),
				floatingErr: errors.New("floating IPs should not be fetched"),
				servers: []*hcloud.Server{{
					ID:       100,
					Name:     "worker",
					Location: &hcloud.Location{Name: "fsn1"},
					ServerType: &hcloud.ServerType{
						Name:   "cpx21",
						Cores:  3,
						Memory: 4,
					},
				}},
			}
		},
	}

	input, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
		CurrencyMode: "net",
		Projects:     []config.Project{{Name: "prod", Token: "token"}},
		Include: config.Include{
			Servers: true,
		},
	})
	if err != nil {
		t.Fatalf("FetchPricingInput() error = %v", err)
	}
	if len(input.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(input.Servers))
	}
	if len(input.Volumes) != 0 || len(input.LoadBalancers) != 0 || len(input.PrimaryIPs) != 0 || len(input.FloatingIPs) != 0 {
		t.Fatalf("unexpected non-server resources: %#v", input)
	}
	if len(input.TrafficObservations) != 0 {
		t.Fatalf("len(TrafficObservations) = %d, want 0 when include.traffic is false", len(input.TrafficObservations))
	}
}

type recordingProjectClient struct {
	fakeProjectClient
	selectors []string
}

func (r *recordingProjectClient) Servers(ctx context.Context, labelSelector string) ([]*hcloud.Server, error) {
	r.selectors = append(r.selectors, labelSelector)
	return r.fakeProjectClient.Servers(ctx, labelSelector)
}

func (r *recordingProjectClient) Volumes(ctx context.Context, labelSelector string) ([]*hcloud.Volume, error) {
	r.selectors = append(r.selectors, labelSelector)
	return r.fakeProjectClient.Volumes(ctx, labelSelector)
}

func (r *recordingProjectClient) LoadBalancers(ctx context.Context, labelSelector string) ([]*hcloud.LoadBalancer, error) {
	r.selectors = append(r.selectors, labelSelector)
	return r.fakeProjectClient.LoadBalancers(ctx, labelSelector)
}

func (r *recordingProjectClient) PrimaryIPs(ctx context.Context, labelSelector string) ([]*hcloud.PrimaryIP, error) {
	r.selectors = append(r.selectors, labelSelector)
	return r.fakeProjectClient.PrimaryIPs(ctx, labelSelector)
}

func (r *recordingProjectClient) FloatingIPs(ctx context.Context, labelSelector string) ([]*hcloud.FloatingIP, error) {
	r.selectors = append(r.selectors, labelSelector)
	return r.fakeProjectClient.FloatingIPs(ctx, labelSelector)
}

func TestFetchPricingInputForwardsLabelSelector(t *testing.T) {
	client := &recordingProjectClient{
		fakeProjectClient: fakeProjectClient{pricing: testPricing()},
	}
	fetcher := Fetcher{
		NewClient: func(string) projectClient {
			return client
		},
	}

	_, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
		CurrencyMode: "net",
		Projects: []config.Project{{
			Name:          "prod",
			Token:         "token",
			LabelSelector: "env=prod",
		}},
	})
	if err != nil {
		t.Fatalf("FetchPricingInput() error = %v", err)
	}

	if len(client.selectors) != 5 {
		t.Fatalf("len(selectors) = %d, want 5", len(client.selectors))
	}
	for idx, selector := range client.selectors {
		if selector != "env=prod" {
			t.Fatalf("selectors[%d] = %q, want env=prod", idx, selector)
		}
	}
}

func closeEnough(left, right float64) bool {
	return math.Abs(left-right) < 0.0000000001
}

func TestFetchPricingInputReturnsProjectErrors(t *testing.T) {
	fetcher := Fetcher{
		NewClient: func(string) projectClient {
			return fakeProjectClient{err: errors.New("boom")}
		},
	}

	_, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
		CurrencyMode: "net",
		Projects:     []config.Project{{Name: "prod", Token: "token"}},
	})
	if err == nil {
		t.Fatal("FetchPricingInput() error = nil, want error")
	}
}

func TestFetchPricingInputReturnsResourceErrors(t *testing.T) {
	tests := []struct {
		name   string
		client fakeProjectClient
		want   string
	}{
		{
			name:   "servers",
			client: fakeProjectClient{pricing: testPricing(), serversErr: errors.New("server list failed")},
			want:   "list servers",
		},
		{
			name:   "volumes",
			client: fakeProjectClient{pricing: testPricing(), volumesErr: errors.New("volume list failed")},
			want:   "list volumes",
		},
		{
			name:   "load_balancers",
			client: fakeProjectClient{pricing: testPricing(), lbsErr: errors.New("lb list failed")},
			want:   "list load balancers",
		},
		{
			name:   "primary_ips",
			client: fakeProjectClient{pricing: testPricing(), primaryErr: errors.New("primary list failed")},
			want:   "list primary IPs",
		},
		{
			name:   "floating_ips",
			client: fakeProjectClient{pricing: testPricing(), floatingErr: errors.New("floating list failed")},
			want:   "list floating IPs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := Fetcher{
				NewClient: func(string) projectClient {
					return tt.client
				},
			}
			_, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
				CurrencyMode: "net",
				Projects:     []config.Project{{Name: "prod", Token: "token"}},
			})
			if err == nil {
				t.Fatal("FetchPricingInput() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("FetchPricingInput() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestFetchPricingInputSkipsIncompleteServers(t *testing.T) {
	fetcher := Fetcher{
		NewClient: func(string) projectClient {
			return fakeProjectClient{
				pricing: testPricing(),
				servers: []*hcloud.Server{
					nil,
					{Name: "missing-type", Location: &hcloud.Location{Name: "fsn1"}},
					{Name: "missing-location", ServerType: &hcloud.ServerType{Name: "cpx21"}},
					{
						ID:       100,
						Name:     "worker",
						Location: &hcloud.Location{Name: "fsn1"},
						ServerType: &hcloud.ServerType{
							Name:   "cpx21",
							Cores:  3,
							Memory: 4,
						},
					},
				},
			}
		},
	}

	input, err := fetcher.FetchPricingInput(context.Background(), &config.Config{
		CurrencyMode: "net",
		Projects:     []config.Project{{Name: "prod", Token: "token"}},
	})
	if err != nil {
		t.Fatalf("FetchPricingInput() error = %v", err)
	}
	if len(input.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(input.Servers))
	}
}

func TestApplicationDefaults(t *testing.T) {
	fetcher := Fetcher{}
	if fetcher.applicationName() != "opencost-hetzner" {
		t.Fatalf("applicationName() = %q", fetcher.applicationName())
	}
	if fetcher.applicationVersion() != "dev" {
		t.Fatalf("applicationVersion() = %q", fetcher.applicationVersion())
	}
}

func TestServerTypeHourlyPriceFindsMatchingTypeAndLocation(t *testing.T) {
	net, gross, err := serverTypeHourlyPrice(testPricing(), "cpx21", "fsn1")
	if err != nil {
		t.Fatalf("serverTypeHourlyPrice() error = %v", err)
	}
	if net != 0.012 {
		t.Fatalf("net = %.4f, want 0.0120", net)
	}
	if gross != 0.0143 {
		t.Fatalf("gross = %.4f, want 0.0143", gross)
	}
}

func TestServerTypeHourlyPriceRejectsMissingLocation(t *testing.T) {
	_, _, err := serverTypeHourlyPrice(testPricing(), "cpx21", "nbg1")
	if err == nil {
		t.Fatal("serverTypeHourlyPrice() error = nil, want error")
	}
}

func TestParsePriceRejectsInvalidDecimal(t *testing.T) {
	_, err := parsePrice("not-a-decimal")
	if err == nil {
		t.Fatal("parsePrice() error = nil, want error")
	}
}

func TestLoadBalancerHourlyPriceRejectsMissingPrice(t *testing.T) {
	_, _, err := loadBalancerHourlyPrice(testPricing(), "lb99", "fsn1")
	if err == nil {
		t.Fatal("loadBalancerHourlyPrice() error = nil, want error")
	}
}

func TestPrimaryIPHourlyPriceRejectsMissingPrice(t *testing.T) {
	_, _, err := primaryIPHourlyPrice(testPricing(), "ipv6", "fsn1")
	if err == nil {
		t.Fatal("primaryIPHourlyPrice() error = nil, want error")
	}
}

func TestFloatingIPHourlyPriceRejectsMissingPrice(t *testing.T) {
	_, _, err := floatingIPHourlyPrice(testPricing(), "ipv4", "fsn1")
	if err == nil {
		t.Fatal("floatingIPHourlyPrice() error = nil, want error")
	}
}

func TestTrafficPriceLookups(t *testing.T) {
	if got := serverPerTBTrafficPrice(testPricing(), "cpx21", "fsn1", "net"); got != 1 {
		t.Fatalf("server net traffic price = %.4f, want 1", got)
	}
	if got := serverPerTBTrafficPrice(testPricing(), "cpx21", "fsn1", "gross"); got != 1.19 {
		t.Fatalf("server gross traffic price = %.4f, want 1.19", got)
	}
	if got := loadBalancerPerTBTrafficPrice(testPricing(), "lb11", "fsn1", "net"); got != 2 {
		t.Fatalf("lb net traffic price = %.4f, want 2", got)
	}
	if got := loadBalancerPerTBTrafficPrice(testPricing(), "lb11", "fsn1", "gross"); got != 2.38 {
		t.Fatalf("lb gross traffic price = %.4f, want 2.38", got)
	}
	if got := serverPerTBTrafficPrice(testPricing(), "missing", "fsn1", "net"); got != 0 {
		t.Fatalf("missing server traffic price = %.4f, want 0", got)
	}
	if got := loadBalancerPerTBTrafficPrice(testPricing(), "missing", "fsn1", "net"); got != 0 {
		t.Fatalf("missing lb traffic price = %.4f, want 0", got)
	}
}

func TestApplicationOverrides(t *testing.T) {
	fetcher := Fetcher{
		ApplicationName:    "custom-name",
		ApplicationVersion: "1.2.3",
	}
	if fetcher.applicationName() != "custom-name" {
		t.Fatalf("applicationName() = %q", fetcher.applicationName())
	}
	if fetcher.applicationVersion() != "1.2.3" {
		t.Fatalf("applicationVersion() = %q", fetcher.applicationVersion())
	}
}

func testPricing() hcloud.Pricing {
	return hcloud.Pricing{
		Volume: hcloud.VolumePricing{
			PerGBMonthly: hcloud.Price{Net: "0.0476", Gross: "0.0566"},
		},
		LoadBalancerTypes: []hcloud.LoadBalancerTypePricing{
			{
				LoadBalancerType: &hcloud.LoadBalancerType{Name: "lb11"},
				Pricings: []hcloud.LoadBalancerTypeLocationPricing{
					{
						Location:     &hcloud.Location{Name: "fsn1"},
						Hourly:       hcloud.Price{Net: "0.0081", Gross: "0.0096"},
						PerTBTraffic: hcloud.Price{Net: "2", Gross: "2.38"},
					},
				},
			},
		},
		PrimaryIPs: []hcloud.PrimaryIPPricing{
			{
				Type: "ipv4",
				Pricings: []hcloud.PrimaryIPTypePricing{
					{
						Location: "fsn1",
						Hourly:   hcloud.PrimaryIPPrice{Net: "0.0015", Gross: "0.0018"},
					},
				},
			},
		},
		FloatingIPs: []hcloud.FloatingIPTypePricing{
			{
				Type: hcloud.FloatingIPTypeIPv6,
				Pricings: []hcloud.FloatingIPTypeLocationPricing{
					{
						Location: &hcloud.Location{Name: "fsn1"},
						Monthly:  hcloud.Price{Net: "0.365", Gross: "0.4344"},
					},
				},
			},
		},
		ServerTypes: []hcloud.ServerTypePricing{
			{
				ServerType: &hcloud.ServerType{Name: "cpx21"},
				Pricings: []hcloud.ServerTypeLocationPricing{
					{
						Location:     &hcloud.Location{Name: "fsn1"},
						Hourly:       hcloud.Price{Net: "0.0120", Gross: "0.0143"},
						PerTBTraffic: hcloud.Price{Net: "1", Gross: "1.19"},
					},
				},
			},
		},
	}
}
