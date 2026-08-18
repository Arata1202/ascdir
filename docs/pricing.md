# Pricing

Pricing uses App Store price-point IDs so the configured value is exact and
doesn't depend on locale-specific currency formatting:

```yaml
pricing:
  base_territory: USA
  scheduled_prices:
    - price_point_id: eyJ...
    - price_point_id: eyJ...
      start_date: "2026-09-01"
      end_date: "2026-12-31"
```

Use a price-point ID returned by the App Store Connect API for the configured
base territory. Dates are optional and use `YYYY-MM-DD`; an end date cannot
precede its start date.

List the valid IDs and their customer prices without changing App Store state:

```sh
ascdir price-points --territory USA
```

Apple exposes scheduled price creation but not deletion through this API.
ascdir therefore treats the schedule as append-only and rejects a local change
that removes an existing remote price instead of silently leaving it behind.

Every pricing update requires a dry-run review and explicit commercial-change
confirmation:

```sh
ascdir push --dry-run
ascdir push --allow-commercial-changes
```
