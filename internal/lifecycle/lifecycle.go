package lifecycle

import (
	"fmt"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

func RestoreDeletedResources(previous, current pricing.Input) pricing.Input {
	current.Volumes = restoreVolumes(previous.Volumes, current.Volumes)
	current.LoadBalancers = restoreLoadBalancers(previous.LoadBalancers, current.LoadBalancers)
	current.PrimaryIPs = restorePrimaryIPs(previous.PrimaryIPs, current.PrimaryIPs)
	current.FloatingIPs = restoreFloatingIPs(previous.FloatingIPs, current.FloatingIPs)
	return current
}

func restoreVolumes(previous, current []pricing.Volume) []pricing.Volume {
	seen := map[string]struct{}{}
	for _, resource := range current {
		seen[resourceKey(resource.ProjectName, resource.ID)] = struct{}{}
	}
	for _, resource := range previous {
		if _, ok := seen[resourceKey(resource.ProjectName, resource.ID)]; ok {
			continue
		}
		resource.Deleted = true
		current = append(current, resource)
	}
	return current
}

func restoreLoadBalancers(previous, current []pricing.LoadBalancer) []pricing.LoadBalancer {
	seen := map[string]struct{}{}
	for _, resource := range current {
		seen[resourceKey(resource.ProjectName, resource.ID)] = struct{}{}
	}
	for _, resource := range previous {
		if _, ok := seen[resourceKey(resource.ProjectName, resource.ID)]; ok {
			continue
		}
		resource.Deleted = true
		current = append(current, resource)
	}
	return current
}

func restorePrimaryIPs(previous, current []pricing.PrimaryIP) []pricing.PrimaryIP {
	seen := map[string]struct{}{}
	for _, resource := range current {
		seen[resourceKey(resource.ProjectName, resource.ID)] = struct{}{}
	}
	for _, resource := range previous {
		if _, ok := seen[resourceKey(resource.ProjectName, resource.ID)]; ok {
			continue
		}
		resource.Deleted = true
		current = append(current, resource)
	}
	return current
}

func restoreFloatingIPs(previous, current []pricing.FloatingIP) []pricing.FloatingIP {
	seen := map[string]struct{}{}
	for _, resource := range current {
		seen[resourceKey(resource.ProjectName, resource.ID)] = struct{}{}
	}
	for _, resource := range previous {
		if _, ok := seen[resourceKey(resource.ProjectName, resource.ID)]; ok {
			continue
		}
		resource.Deleted = true
		current = append(current, resource)
	}
	return current
}

func resourceKey(projectName string, id int64) string {
	return fmt.Sprintf("%s:%d", projectName, id)
}
