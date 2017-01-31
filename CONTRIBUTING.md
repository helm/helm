# Contributing Guidelines

The Kubernetes Helm project accepts contributions via GitHub pull requests. This document outlines the process to help get your contribution accepted.

## Reporting a Security Issue

Most of the time, when you find a bug in Helm, it should be reported
using [GitHub issues](github.com/kubernetes/helm/issues). However, if
you are reporting a _security vulnerability_, please email a report to
[helm-security@deis.com](mailto:helm-security@deis.com). This will give
us a chance to try to fix the issue before it is exploited in the wild.

## Contributor License Agreements

We'd love to accept your patches! Before we can take them, we have to jump a
couple of legal hurdles.

The Cloud Native Computing Foundation (CNCF) CLA [must be signed](https://github.com/kubernetes/community/blob/master/CLA.md) by all contributors.
Please fill out either the individual or corporate Contributor License
Agreement (CLA).

Once you are CLA'ed, we'll be able to accept your pull requests. For any issues that you face during this process,
please add a comment [here](https://github.com/kubernetes/kubernetes/issues/27796) explaining the issue and we will help get it sorted out.

***NOTE***: Only original source code from you and other people that have
signed the CLA can be accepted into the repository. This policy does not
apply to [third_party](third_party/) and [vendor](vendor/).

## How to Submit a Proposal

Submit proposals through the issue queue. The title of your issue should start with `PROPOSAL:`.

## How to Contribute a Patch

1. If you haven't already done so, sign a Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

Coding conventions and standards are explained in the official
developer docs:
https://github.com/kubernetes/helm/blob/master/docs/developers.md

### Merge Approval

Helm collaborators may add "LGTM" (Looks Good To Me) or an equivalent comment to indicate that a PR is acceptable. Any change requires at least one LGTM.  No pull requests can be merged until at least one Helm collaborator signs off with an LGTM.

If the PR is from a Helm collaborator, then he or she should be the one to merge and close it. This keeps the commit stream clean and gives the collaborator the benefit of revisiting the PR before deciding whether or not to merge the changes.

## Support Channels

Whether you are a user or contributor, official support channels include:

- GitHub issues: https://github.com/kubenetes/helm/issues/new
- Slack: #Helm room in the [Kubernetes Slack](http://slack.kubernetes.io/)

Before opening a new issue or submitting a new pull request, it's helpful to search the project - it's likely that another user has already reported the issue you're facing, or it's a known issue that we're already aware of.
