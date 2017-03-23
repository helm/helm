# Requirements Files

This section of the guide covers best practices for `requirements.yaml` files.

## Versions

Where possible, use version ranges instead of pinning to an exact version. The suggested default is to use a patch-level version match:

```yaml
version: ^1.2.0
```

This will match version 1.2.0 and any patches to that release (1.2.1, 1.2.999, and so on)

### Repository URLs

Where possible, use `https://` repository URLs, followed by `http://` URLs.

If the repository has been added to the repository index file, the repository name can be used as an alias of URL. Use `alias:` or `@` followed by repository names.

File URLs (`file://...`) are considered a "special case" for charts that are assembled by a fixed deployment pipeline. Charts that use `file://` in a `requirements.yaml` file are not allowed in the official Helm repository.

## Conditions and Tags

Conditions or tags should be added to any dependencies that _are optional_.

The preferred form of a condition is:

```yaml
condition: somechart.enabled
```

Where `somechart` is the chart name of the dependency.

When multiple subcharts (dependencies) together provide an optional or swappable feature, those charts should share the same tags.

For example, if both `nginx` and `memcached` together provided performance optimizations for the main app in the chart, and were required to both be present when that feature is enabled, then they might both have a
tags section like this:

```
tags:
  - webaccelerator
```

This allows a user to turn that feature on and off with one tag.