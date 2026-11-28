# Values Files

This document covers common patterns and troubleshooting for Helm values files.

## Troubleshooting: "wrong type for value; expected table"

This error occurs when a Helm values file sets a key to a scalar value
(a string, number, or boolean) but a subchart or parent chart expects
that key to be a mapping (object/table).

For example, if `values.yaml` in a subchart defines:

```yaml
service:
  port: 80
```

And a parent chart overrides it with:

```yaml
service: "disabled"
```

Helm will throw an error because `service` is expected to be a mapping,
not a string.

The underlying coalesce logic produces warnings such as:

- `warning: destination for <key> is a table. Ignoring non-table value (<value>)`
- `warning: cannot overwrite table with non table for <key> (<value>)`

**Fix:** Ensure the override preserves the type. To effectively disable a
service, use a flag inside the mapping:

```yaml
service:
  enabled: false
  port: 80
```

This keeps `service` as a table (mapping) so Helm can merge the values
correctly.
