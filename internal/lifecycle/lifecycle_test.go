package lifecycle

import (
	"testing"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

func TestRestoreDeletedResourcesAddsMissingPreviousResources(t *testing.T) {
	previous := pricing.Input{
		Volumes: []pricing.Volume{{
			ID:            10,
			Name:          "deleted-volume",
			HourlyCostNet: 0.01,
		}},
		LoadBalancers: []pricing.LoadBalancer{{
			ID:            20,
			Name:          "deleted-lb",
			HourlyCostNet: 0.02,
		}},
		PrimaryIPs: []pricing.PrimaryIP{{
			ID:            30,
			Name:          "deleted-primary",
			HourlyCostNet: 0.03,
		}},
		FloatingIPs: []pricing.FloatingIP{{
			ID:            40,
			Name:          "deleted-floating",
			HourlyCostNet: 0.04,
		}},
	}

	got := RestoreDeletedResources(previous, pricing.Input{})

	if len(got.Volumes) != 1 || !got.Volumes[0].Deleted {
		t.Fatalf("Volumes = %#v, want deleted previous volume", got.Volumes)
	}
	if len(got.LoadBalancers) != 1 || !got.LoadBalancers[0].Deleted {
		t.Fatalf("LoadBalancers = %#v, want deleted previous load balancer", got.LoadBalancers)
	}
	if len(got.PrimaryIPs) != 1 || !got.PrimaryIPs[0].Deleted {
		t.Fatalf("PrimaryIPs = %#v, want deleted previous primary IP", got.PrimaryIPs)
	}
	if len(got.FloatingIPs) != 1 || !got.FloatingIPs[0].Deleted {
		t.Fatalf("FloatingIPs = %#v, want deleted previous floating IP", got.FloatingIPs)
	}
}

func TestRestoreDeletedResourcesDoesNotDuplicateCurrentResources(t *testing.T) {
	previous := pricing.Input{
		Volumes: []pricing.Volume{{ID: 10, Name: "data"}},
	}
	current := pricing.Input{
		Volumes: []pricing.Volume{{ID: 10, Name: "data"}},
	}

	got := RestoreDeletedResources(previous, current)
	if len(got.Volumes) != 1 {
		t.Fatalf("len(Volumes) = %d, want 1", len(got.Volumes))
	}
	if got.Volumes[0].Deleted {
		t.Fatal("current volume marked deleted")
	}
}

func TestRestoreDeletedResourcesKeysByProjectAndID(t *testing.T) {
	previous := pricing.Input{
		Volumes: []pricing.Volume{{
			ProjectName: "prod",
			ID:          10,
			Name:        "prod-data",
		}},
	}
	current := pricing.Input{
		Volumes: []pricing.Volume{{
			ProjectName: "stage",
			ID:          10,
			Name:        "stage-data",
		}},
	}

	got := RestoreDeletedResources(previous, current)
	if len(got.Volumes) != 2 {
		t.Fatalf("len(Volumes) = %d, want 2", len(got.Volumes))
	}
	if got.Volumes[1].ProjectName != "prod" || !got.Volumes[1].Deleted {
		t.Fatalf("restored volume = %#v, want deleted prod volume", got.Volumes[1])
	}
}
