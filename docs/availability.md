# Availability and preorders

Manage only the territories you list under `availability`:

```yaml
availability:
  available_in_new_territories: false
  territories:
    JPN:
      available: true
      release_date: "2026-09-01"
      pre_order_enabled: false
    USA:
      available: false
```

Territory keys are three-letter App Store territory IDs. Dates use
`YYYY-MM-DD`. Omit a field to leave it unmanaged.

For an existing availability resource, App Store Connect doesn't provide an
update operation for `available_in_new_territories`; ascdir reports an error
instead of silently ignoring a requested change. The field is used when the
availability resource needs to be created.

Enabling a preorder can affect the public storefront and requires explicit
confirmation after reviewing the dry run:

```sh
ascdir push --dry-run
ascdir push --allow-irreversible
```
