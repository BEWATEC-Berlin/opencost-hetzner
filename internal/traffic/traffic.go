package traffic

import (
	"fmt"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

const bytesPerTB = 1_000_000_000_000

func ComputeCosts(previous, current pricing.Input) []pricing.TrafficCost {
	previousByKey := map[string]pricing.TrafficObservation{}
	for _, observation := range previous.TrafficObservations {
		previousByKey[key(observation.ProjectName, observation.ResourceType, observation.ID)] = observation
	}

	costs := []pricing.TrafficCost{}
	for _, observation := range current.TrafficObservations {
		currentBillable := billableBytes(observation)
		previousBillable := uint64(0)
		if previousObservation, ok := previousByKey[key(observation.ProjectName, observation.ResourceType, observation.ID)]; ok {
			previousBillable = billableBytes(previousObservation)
		}
		if currentBillable <= previousBillable {
			continue
		}

		deltaBytes := currentBillable - previousBillable
		usageTB := float64(deltaBytes) / bytesPerTB
		costs = append(costs, pricing.TrafficCost{
			ProjectName:      observation.ProjectName,
			ResourceType:     "traffic",
			ID:               observation.ID,
			Name:             observation.Name,
			Location:         observation.Location,
			UsageTB:          usageTB,
			CostNet:          usageTB * observation.PerTBTrafficNet,
			CostGross:        usageTB * observation.PerTBTrafficGross,
			SourceResource:   observation.ResourceType,
			SourceProviderID: fmt.Sprintf("%d", observation.ID),
		})
	}
	return costs
}

func billableBytes(observation pricing.TrafficObservation) uint64 {
	if observation.OutgoingBytes <= observation.IncludedTrafficBytes {
		return 0
	}
	return observation.OutgoingBytes - observation.IncludedTrafficBytes
}

func key(projectName, resourceType string, id int64) string {
	return fmt.Sprintf("%s:%s:%d", projectName, resourceType, id)
}
