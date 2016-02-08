Testing via Helm
================

Problem Summary
---------------

Currently in helm/charts we have a simple way of health-checking charts which
checks that all pods in the chart reach "Running" state on a running Kubernetes
cluster. In order to build a system that holds chart quality at the forefront,
we need to introduce an easy way to create and run potentially more complex
application-centric tests.

Proposed Solution
-----------------

#### User Experience

```
# helm/dm UX for installing charts stays the same

# addition to UX giving easy-to-run tests:
helm test deis

```

#### Helm's new `test` Command

This command uses test logic located in `<chart name>/tests/` to ensure that
the chart is operating as intended in an end to end or system test sort of
fashion.

The command is fairly simple - all it does is load a set of charts into a
Kubernetes cluster (might be the same or different as the deployed chart).
What usually makes sense to have is a one-off pod that runs to completion and
exits with a certain exit code: non-zero signifying failure or zero signifying
success. The pod is not automatically restarted so that humans or automated
tools can inspect the results which might be a log, test artifact, or something
else. `helm test` should therefore be able to be rerun the tests over and over and
not be hindered by an existing similarly-named set of pods, rcs, or services.

By forcing tests to also be containerized and Kubernetes-ready we have the
benefit of having a single and easily understandable entry point - it's just
another set of Kubernetes components.

#### Modifications to Chart Structure

```
ROOT/
  Chart.yaml
  LICENSE
  README.md
  ...
  tests/
    Chart.yaml
    templates/
      some.yaml
      some.jinja
      some.jinja.schema
```
