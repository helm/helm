# Release Checklist

**IMPORTANT**: If your experience deviates from this document, please document the changes to keep it up-to-date.

## A Maintainer's Guide to Releasing Helm

So you're in charge of a new release for helm? Cool. Here's what to do...

![TODO: Nothing](../images/nothing.png)

Just kidding! :trollface:

All releases will be of the form vX.Y.Z where X is the major version number, Y is the minor version number and Z is the patch release number. This project strictly follows [semantic versioning](http://semver.org/) so following this step is critical.

It is important to note that this document assumes that the git remote in your repository that corresponds to "https://github.com/kubernetes/helm" is named "upstream". If yours is not (for example, if you've chosen to name it "origin" or something similar instead), be sure to adjust the listed snippets for your local environment accordingly. If you are not sure what your upstream remote is named, use a command like `git remote -v` to find out.

If you don't have an upstream remote, you can add one easily using something like:

```shell
git remote add upstream git@github.com:kubernetes/helm.git
```

In this doc, we are going to reference a few environment variables as well, which you may want to set for convenience. For major/minor releases, use the following:

```shell
export RELEASE_NAME=vX.Y.0
export RELEASE_BRANCH_NAME="release-$RELEASE_NAME"
export RELEASE_CANDIDATE_NAME="$RELEASE_NAME-rc1"
```

If you are creating a patch release, you may want to use the following instead:

```shell
export PREVIOUS_PATCH_RELEASE=vX.Y.Z
export RELEASE_NAME=vX.Y.Z+1
export RELEASE_BRANCH_NAME="release-X.Y"
export RELEASE_CANDIDATE_NAME="$RELEASE_NAME-rc1"
```

## 1. Create the Release Branch

### Major/Minor Releases

Major releases are for new feature additions and behavioral changes *that break backwards compatibility*. Minor releases are for new feature additions that do not break backwards compatibility. To create a major or minor release, start by creating a `release-vX.Y.0` branch from master.

```shell
git fetch upstream
git checkout upstream/master
git checkout -b $RELEASE_BRANCH_NAME
```

This new branch is going to be the base for the release, which we are going to iterate upon later.

### Patch releases

Patch releases are a few critical cherry-picked fixes to existing releases. Start by creating a `release-vX.Y.Z` branch from the latest patch release.

```shell
git fetch upstream --tags
git checkout $PREVIOUS_PATCH_RELEASE
git checkout -b $RELEASE_BRANCH_NAME
```

From here, we can cherry-pick the commits we want to bring into the patch release:

```shell
# get the commits ids we want to cherry-pick
git log --oneline
# cherry-pick the commits starting from the oldest one, without including merge commits
git cherry-pick -x <commit-id>
git cherry-pick -x <commit-id>
```

This new branch is going to be the base for the release, which we are going to iterate upon later.

## 2. Change the Version Number in Git

When doing a minor release, make sure to update pkg/version/version.go with the new release version.

```shell
$ git diff pkg/version/version.go
diff --git a/pkg/version/version.go b/pkg/version/version.go
index 2109a0a..6f5a1a4 100644
--- a/pkg/version/version.go
+++ b/pkg/version/version.go
@@ -26,7 +26,7 @@ var (
        // Increment major number for new feature additions and behavioral changes.
        // Increment minor number for bug fixes and performance enhancements.
        // Increment patch number for critical fixes to existing releases.
-       Version = "v2.6"
+       Version = "v2.7"

        // BuildMetadata is extra build time data
        BuildMetadata = "unreleased"
```

The README stores links to the latest release for helm. We want to change the version to the first release candidate which we are releasing (more on that in step 5).

```shell
$ git diff README.md
diff --git a/README.md b/README.md
index 022afd79..547839e2 100644
--- a/README.md
+++ b/README.md
@@ -34,10 +34,10 @@ Think of it like apt/yum/homebrew for Kubernetes.

 Binary downloads of the Helm client can be found at the following links:

-- [OSX](https://kubernetes-helm.storage.googleapis.com/helm-v2.7.0-darwin-amd64.tar.gz)
-- [Linux](https://kubernetes-helm.storage.googleapis.com/helm-v2.7.0-linux-amd64.tar.gz)
-- [Linux 32-bit](https://kubernetes-helm.storage.googleapis.com/helm-v2.7.0-linux-386.tar.gz)
-- [Windows](https://kubernetes-helm.storage.googleapis.com/helm-v2.7.0-windows-amd64.tar.gz)
+- [OSX](https://kubernetes-helm.storage.googleapis.com/helm-v2.8.0-darwin-amd64.tar.gz)
+- [Linux](https://kubernetes-helm.storage.googleapis.com/helm-v2.8.0-linux-amd64.tar.gz)
+- [Linux 32-bit](https://kubernetes-helm.storage.googleapis.com/helm-v2.8.0-linux-386.tar.gz)
+- [Windows](https://kubernetes-helm.storage.googleapis.com/helm-v2.8.0-windows-amd64.tar.gz)

 Unpack the `helm` binary and add it to your PATH and you are good to go!
 macOS/[homebrew](https://brew.sh/) users can also use `brew install kubernetes-helm`.
```

For patch releases, the old version number will be the latest patch release, so just bump the patch number, incrementing Z by one.

```shell
git add .
git commit -m "bump version to $RELEASE_CANDIDATE_NAME"
```

## 3. Commit and Push the Release Branch

In order for others to start testing, we can now push the release branch upstream and start the test process.

```shell
git push upstream $RELEASE_BRANCH_NAME
```

Make sure to check [helm on CircleCI](https://circleci.com/gh/kubernetes/helm) and make sure the release passed CI before proceeding.

If anyone is available, let others peer-review the branch before continuing to ensure that all the proper changes have been made and all of the commits for the release are there.

## 4. Create a Release Candidate

Now that the release branch is out and ready, it is time to start creating and iterating on release candidates.

```shell
git tag --sign --annotate "${RELEASE_CANDIDATE_NAME}" --message "Helm release ${RELEASE_CANDIDATE_NAME}"
git push upstream $RELEASE_CANDIDATE_NAME
```

CircleCI will automatically create a tagged release image and client binary to test with.

For testers, the process to start testing after CircleCI finishes building the artifacts involves the following steps to grab the client from Google Cloud Storage:

linux/amd64, using /bin/bash:

```shell
wget https://kubernetes-helm.storage.googleapis.com/helm-$RELEASE_CANDIDATE_NAME-linux-amd64.tar.gz
```

darwin/amd64, using Terminal.app:

```shell
wget https://kubernetes-helm.storage.googleapis.com/helm-$RELEASE_CANDIDATE_NAME-darwin-amd64.tar.gz
```

windows/amd64, using PowerShell:

```shell
PS C:\> Invoke-WebRequest -Uri "https://kubernetes-helm.storage.googleapis.com/helm-$RELEASE_CANDIDATE_NAME-windows-amd64.tar.gz" -OutFile "helm-$ReleaseCandidateName-windows-amd64.tar.gz"
```

Then, unpack and move the binary to somewhere on your $PATH, or move it somewhere and add it to your $PATH (e.g. /usr/local/bin/helm for linux/macOS, C:\Program Files\helm\helm.exe for Windows).

## 5. Iterate on Successive Release Candidates

Spend several days explicitly investing time and resources to try and break helm in every possible way, documenting any findings pertinent to the release. This time should be spent testing and finding ways in which the release might have caused various features or upgrade environments to have issues, not coding. During this time, the release is in code freeze, and any additional code changes will be pushed out to the next release.

During this phase, the $RELEASE_BRANCH_NAME branch will keep evolving as you will produce new release candidates. The frequency of new candidates is up to the release manager: use your best judgement taking into account the severity of reported issues, testers' availability, and the release deadline date. Generally speaking, it is better to let a release roll over the deadline than to ship a broken release.

Each time you'll want to produce a new release candidate, you will start by adding commits to the branch by cherry-picking from master:

```shell
git cherry-pick -x <commit_id>
```

You will also want to update the release version number and the CHANGELOG as we did in steps 2 and 3 as separate commits.

After that, tag it and notify users of the new release candidate:

```shell
export RELEASE_CANDIDATE_NAME="$RELEASE_NAME-rc2"
git tag --sign --annotate "${RELEASE_CANDIDATE_NAME}" --message "Helm release ${RELEASE_CANDIDATE_NAME}"
git push upstream $RELEASE_CANDIDATE_NAME
```

From here on just repeat this process, continuously testing until you're happy with the release candidate.

## 6. Finalize the Release

When you're finally happy with the quality of a release candidate, you can move on and create the real thing. Double-check one last time to make sure eveything is in order, then finally push the release tag.

```shell
git checkout $RELEASE_BRANCH_NAME
git tag --sign --annotate "${RELEASE_NAME}" --message "Helm release ${RELEASE_NAME}"
git push upstream $RELEASE_NAME
```

## 7. Write the Release Notes

We will auto-generate a changelog based on the commits that occurred during a release cycle, but it is usually more beneficial to the end-user if the release notes are hand-written by a human being/marketing team/dog.

If you're releasing a major/minor release, listing notable user-facing features is usually sufficient. For patch releases, do the same, but make note of the symptoms and who is affected.

An example release note for a minor release would look like this:

```markdown
## vX.Y.Z

Helm vX.Y.Z is a feature release. This release, we focused on <insert focal point>. Users are encouraged to upgrade for the best experience.

The community keeps growing, and we'd love to see you there.

- Join the discussion in [Kubernetes Slack](https://slack.k8s.io/):
  - `#helm-users` for questions and just to hang out
  - `#helm-dev` for discussing PRs, code, and bugs
- Hang out at the Public Developer Call: Thursday, 9:30 Pacific via [Zoom](https://zoom.us/j/4526666954)
- Test, debug, and contribute charts: [GitHub/kubernetes/charts](https://github.com/kubernetes/charts)

## Installation and Upgrading

Download Helm X.Y. The common platform binaries are here:

- [OSX](https://storage.googleapis.com/kubernetes-helm/helm-vX.Y.Z-darwin-amd64.tar.gz)
- [Linux](https://storage.googleapis.com/kubernetes-helm/helm-vX.Y.Z-linux-amd64.tar.gz)
- [Windows](https://storage.googleapis.com/kubernetes-helm/helm-vX.Y.Z-windows-amd64.tar.gz)

Once you have the client installed, upgrade Tiller with `helm init --upgrade`.

The [Quickstart Guide](https://docs.helm.sh/using_helm/#quickstart-guide) will get you going from there. For **upgrade instructions** or detailed installation notes, check the [install guide](https://docs.helm.sh/using_helm/#installing-helm). You can also use a [script to install](https://raw.githubusercontent.com/kubernetes/helm/master/scripts/get) on any system with `bash`.

## What's Next

- vX.Y.Z+1 will contain only bug fixes.
- vX.Y+1.Z is the next feature release. This release will focus on ...

## Changelog

- chore(*): bump version to v2.7.0 08c1144f5eb3e3b636d9775617287cc26e53dba4 (Adam Reese)
- fix circle not building tags f4f932fabd197f7e6d608c8672b33a483b4b76fa (Matthew Fisher)
```

The changelog at the bottom of the release notes can be generated with this command:

```shell
PREVIOUS_RELEASE=vX.Y.Z
git log --no-merges --pretty=format:'- %s %H (%aN)' $RELEASE_NAME $PREVIOUS_RELEASE
```

Once finished, go into GitHub and edit the release notes for the tagged release with the notes written here.

## 9. Evangelize

Congratulations! You're done. Go grab yourself a $DRINK_OF_CHOICE. You've earned it.

After enjoying a nice $DRINK_OF_CHOICE, go forth and announce the glad tidings of the new release in Slack and on Twitter. You should also notify any key partners in the helm community such as the homebrew formula maintainers, the owners of incubator projects (e.g. ChartMuseum) and any other interested parties.

Optionally, write a blog post about the new release and showcase some of the new features on there!
