# The Chart Best Practices Guide

This guide covers the Helm Team's considered best practices for creating charts.
It focuses on how charts should be structured.

We focus primarily on best practices for charts that may be publicly deployed.
We know that many charts are for internal-use only, and authors of such charts
may find that their internal interests override our suggestions here.

## Table of Contents

- [General Conventions](conventions.md): Learn about general chart conventions.
- [Values Files](values.md): See the best practices for structuring `values.yaml`.
- [Templates](templates.md): Learn some of the best techniques for writing templates.
- [Requirements](requirements.md): Follow best practices for `requirements.yaml` files.
- [Labels and Annotations](labels.md): Helm has a _heritage_ of labeling and annotating.
- Kubernetes Resources:
	- [Pods and Pod Specs](pods.md): See the best practices for working with pod specifications.
	- [Third Party Resources](third_party_resources.md): Third Party Resources (TPRs) have their own associated best practices.

