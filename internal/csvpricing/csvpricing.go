package csvpricing

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/bewatec/opencost-hetzner/internal/pricing"
)

const (
	nodeInstanceIDField = "spec.providerID"
	pvInstanceIDField   = "spec.storageClassName"
	version             = "v1"
	hoursPerMonth       = 730
)

type Input struct {
	CurrencyMode   string
	Servers        []pricing.Server
	Volume         pricing.VolumePricing
	StorageClasses []string
}

type Row struct {
	EndTimestamp      string
	InstanceID        string
	Region            string
	AssetClass        string
	InstanceIDField   string
	InstanceType      string
	MarketPriceHourly string
	Version           string
}

func GenerateRows(input Input) ([]Row, error) {
	if input.CurrencyMode == "" {
		input.CurrencyMode = "net"
	}
	if input.CurrencyMode != "net" && input.CurrencyMode != "gross" {
		return nil, fmt.Errorf("currency mode must be net or gross")
	}

	rows := make([]Row, 0, len(input.Servers)+len(input.StorageClasses))
	for _, server := range input.Servers {
		cost := server.HourlyCostNet
		if input.CurrencyMode == "gross" {
			cost = server.HourlyCostGross
		}
		if cost <= 0 {
			return nil, fmt.Errorf("server %q has invalid hourly cost %.10f", server.Name, cost)
		}
		rows = append(rows, Row{
			InstanceID:        fmt.Sprintf("hcloud://%d", server.ID),
			Region:            server.Location,
			AssetClass:        "node",
			InstanceIDField:   nodeInstanceIDField,
			InstanceType:      server.Type,
			MarketPriceHourly: formatPrice(cost),
			Version:           version,
		})
	}

	storageMonthly := input.Volume.PerGBMonthNet
	if input.CurrencyMode == "gross" {
		storageMonthly = input.Volume.PerGBMonthGross
	}
	if storageMonthly > 0 {
		for _, storageClass := range input.StorageClasses {
			rows = append(rows, Row{
				InstanceID:        storageClass,
				AssetClass:        "pv",
				InstanceIDField:   pvInstanceIDField,
				MarketPriceHourly: formatPrice(storageMonthly / hoursPerMonth),
				Version:           version,
			})
		}
	}

	return rows, nil
}

func Write(w io.Writer, rows []Row) error {
	writer := csv.NewWriter(w)
	header := []string{
		"EndTimestamp",
		"InstanceID",
		"Region",
		"AssetClass",
		"InstanceIDField",
		"InstanceType",
		"MarketPriceHourly",
		"Version",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			row.EndTimestamp,
			row.InstanceID,
			row.Region,
			row.AssetClass,
			row.InstanceIDField,
			row.InstanceType,
			row.MarketPriceHourly,
			row.Version,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func formatPrice(value float64) string {
	formatted := strconv.FormatFloat(value, 'f', 10, 64)
	for len(formatted) > 0 && formatted[len(formatted)-1] == '0' {
		formatted = formatted[:len(formatted)-1]
	}
	if len(formatted) > 0 && formatted[len(formatted)-1] == '.' {
		formatted = formatted[:len(formatted)-1]
	}
	return formatted
}
