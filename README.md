# OpenCost Hetzner

OpenCost Hetzner generates Hetzner Cloud pricing data that can be consumed by OpenCost.

The current implementation focuses on two practical outputs:

- exact-node CSV pricing for OpenCost workload allocation by namespace and pod
- custom-cost JSON artifacts for Hetzner resources that do not naturally map to Kubernetes node or persistent volume allocation

Costs default to Hetzner net prices.

## Status

This project is early and the current integration is a generator, not a native OpenCost cloud provider.

Implemented:

- Hetzner Cloud pricing lookup
- multi-project configuration
- exact-node OpenCost CSV pricing for Kubernetes nodes
- OpenCost CSV persistent volume pricing by storage class
- custom-cost JSON for volumes, load balancers, primary IPs, floating IPs, and traffic overage
- snapshot-based traffic delta calculation
- previous-snapshot reconstruction for deleted non-node resources
- per-project Hetzner label selectors
- resource include flags

Not implemented yet:

- native OpenCost cloud provider support
- running OpenCost custom-cost plugin RPC server
- object storage usage import
- Storage Box import
- invoice-grade monthly cap reconciliation

## Usage

Create a config file from the example:

```bash
cp hetzner_config.example.json hetzner_config.json
```

Edit `hetzner_config.json` and add read-only Hetzner Cloud API tokens.

Generate exact-node CSV pricing:

```bash
go run ./cmd/opencost-hetzner-pricing \
  -config ./hetzner_config.json \
  -format csv > hetzner-opencost-prices.csv
```

Generate blended OpenCost custom pricing Helm values:

```bash
go run ./cmd/opencost-hetzner-pricing \
  -config ./hetzner_config.json \
  -format helm-values
```

Generate custom-cost JSON for non-node resources:

```bash
go run ./cmd/opencost-hetzner-pricing \
  -config ./hetzner_config.json \
  -format custom-costs > hetzner-custom-costs.json
```

## Configuration

Example:

```json
{
  "log_level": "info",
  "currency_mode": "net",
  "projects": [
    {
      "name": "production",
      "token": "<read-only-hcloud-token>",
      "label_selector": ""
    }
  ],
  "persistence": {
    "enabled": true,
    "path": "/var/lib/opencost-hetzner"
  },
  "include": {
    "servers": true,
    "volumes": true,
    "load_balancers": true,
    "primary_ips": true,
    "floating_ips": true,
    "traffic": true
  },
  "kubernetes": {
    "storage_classes": ["hcloud-volumes"]
  }
}
```

Notes:

- `currency_mode` can be `net` or `gross`; default is `net`.
- Use read-only Hetzner Cloud API tokens.
- `label_selector` is forwarded to Hetzner Cloud list calls.
- `persistence.enabled` is recommended for traffic deltas and deleted-resource reconstruction.
- Do not commit `hetzner_config.json`.

## OpenCost CSV Provider

OpenCost can read exact-node prices from a CSV file. Hetzner Cloud Controller Manager sets Kubernetes node provider IDs in this form:

```text
hcloud://<server-id>
```

The generated CSV uses that value as the node key:

```csv
EndTimestamp,InstanceID,Region,AssetClass,InstanceIDField,InstanceType,MarketPriceHourly,Version
,hcloud://123456,fsn1,node,spec.providerID,cpx31,0.024,v1
```

Persistent volume rows are generated for configured storage classes:

```csv
,hcloud-volumes,,pv,spec.storageClassName,,0.0000783562,v1
```

One deployment pattern is to put the generated CSV in a ConfigMap and mount it into the OpenCost exporter container:

```bash
kubectl -n opencost create configmap hetzner-opencost-prices \
  --from-file=hetzner-opencost-prices.csv \
  --dry-run=client -o yaml | kubectl apply -f -
```

Example Helm values shape:

```yaml
opencost:
  exporter:
    csv_path: /var/opencost/hetzner-opencost-prices.csv
    extraEnv:
      USE_CSV_PROVIDER: "true"
      USE_CUSTOM_PROVIDER: "true"
      CSV_PATH: /var/opencost/hetzner-opencost-prices.csv
    extraVolumeMounts:
      - name: hetzner-pricing
        mountPath: /var/opencost

extraVolumes:
  - name: hetzner-pricing
    configMap:
      name: hetzner-opencost-prices
```

## Custom Costs

The `custom-costs` output is intended for Hetzner resources that should not be folded into node CPU/RAM prices:

- unattached or standalone volumes
- load balancers
- primary IPs
- floating IPs
- traffic overage records

When persistence is enabled, the generator stores a token-free snapshot at:

```text
<persistence.path>/latest.json
```

On the next run it compares the previous and current observations and emits:

- traffic records for new billable overage
- deleted-resource records for resources that existed in the previous snapshot but disappeared from the current Hetzner API response

Deleted resources are labeled with `lifecycle=deleted`.

## Service Coverage

| Cost class | Current path | Status |
| --- | --- | --- |
| Kubernetes node CPU/RAM | OpenCost CSV provider exact node rows | Implemented |
| Kubernetes persistent volumes | OpenCost CSV provider PV rows by storage class | Implemented |
| Volumes outside Kubernetes PV mapping | Custom-cost JSON | Implemented |
| Load balancers | Custom-cost JSON | Implemented |
| Traffic overage | Snapshot-based custom-cost JSON | Implemented |
| Primary/Floating IPs | Custom-cost JSON | Implemented |
| Object storage | Separate usage import needed | Not implemented |
| Storage Boxes | Separate Hetzner API/import needed | Not implemented |

## Development

Run tests:

```bash
go test ./...
```

Build the generator:

```bash
go build -o opencost-hetzner-pricing ./cmd/opencost-hetzner-pricing
```

## Security

- Use read-only Hetzner Cloud API tokens.
- Never commit real tokens or generated config containing tokens.
- Avoid write-scoped Hetzner API tokens.
- Treat generated pricing outputs as operational data before publishing them.

