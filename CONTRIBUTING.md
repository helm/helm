# Contributing Guidelines

The Kubernetes Helm project accepts contributions via GitHub pull requests. This document outlines the process to help get your contribution accepted.

## Contributor License Agreements

We'd love to accept your patches! Before we can take them, we have to jump a couple of legal hurdles.

Please fill out either the individual or corporate Contributor License Agreement (CLA).

  * If you are an individual writing original source code and you're sure you own the intellectual property, then you'll need to sign an [individual CLA](http://code.google.com/legal/individual-cla-v1.0.html).
    * If you work for a company that wants to allow you to contribute your work, then you'll need to sign a [corporate CLA](http://code.google.com/legal/corporate-cla-v1.0.html).

Follow either of the two links above to access the appropriate CLA and instructions for how to sign and return it. Once we receive it, we'll be able to accept your pull requests.

***NOTE***: Only original source code from you and other people that have signed the CLA can be accepted into the main repository.

## How to Contribute A Patch

### Overview

1. Submit an issue describing your proposed change to the repo in question.
1. The repo owner will respond to your issue promptly.
1. If your proposed change is accepted, and you haven't already done so, sign a Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

### Design Document

If the change you are proposing is substantial, before opening a pull request, ensure you reference a design document that can be discussed in the open.

### Single Issue

When fixing or implementing a GitHub issue, resist the temptation to refactor nearby code or to fix that potential bug you noticed. Instead, open a new pull request just for that change.

It's hard to reach agreement on the merit of a PR when it isn't focused. Keeping concerns separated allows pull requests to be tested, reviewed, and merged more quickly.

Squash and rebase the commit or commits in your pull request into logical units of work with `git`. Include tests and documentation changes in the same commit, so that a revert would remove all traces of the feature or fix.

Most pull requests will reference a GitHub issue. In the PR description–not in the commit itself–include a line such as "Closes #1234". The issue referenced will then be closed when your PR is merged.

### Include Tests & Documentation

If you change or add functionality, your changes should include the necessary tests to prove that it works. While working on local code changes, always run the tests.  Any change that could affect a user's experience also needs a change or addition to the relevant documentation.  

Pull requests that do not include sufficient tests or documentation will be rejected.

### Code Standards

Go code should always be run through `gofmt` on the default settings. Lines of code may be up to 99 characters long. Documentation strings and tests are required for all public methods. Use of third-party go packages should be minimal, but when doing so, vendor code using [Glide](http://glide.sh/).

### Merge Approval

Helm collaborators may add "LGTM" (Looks Good To Me) or an equivalent comment to indicate that a PR is acceptable. Any code change (other than a simple typo fix or one-line documentation change) requires at least one LGTM.  No pull requests can be merged until at least one Helm collaborator signs off with an LGTM.

If the PR is from a Helm collaborator, then he or she should be the one to merge and close it. This keeps the commit stream clean and gives the collaborator the benefit of revisiting the PR before deciding whether or not to merge the changes.

## Support Channels

Whether you are a user or contributor, official support channels include:

- GitHub issues: https://github.com/kubenetes/helm/issues/new
- Slack: #Helm room in the [Kubernetes Slack](http://slack.kubernetes.io/)

Before opening a new issue or submitting a new pull request, it's helpful to search the project - it's likely that another user has already reported the issue you're facing, or it's a known issue that we're already aware of.
