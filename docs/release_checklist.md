# Release Checklist

**IMPORTANT**: If your experience deviates from this document, please document the changes to keep it up-to-date.

## Release Meetings
As part of the release process, two of the weekly developer calls will be
co-opted as "release meetings."

### Start of the Release Cycle
The first developer call after a release will be used as the release meeting to
start the next release cycle. During this meeting, the following items must be
identified:

- Release date
- Goals/Objectives for this release
- The release manager (basically whoever is going to cut the release)
- Any other important details for the community

All of this information should be added to the GitHub milestone for the given
release. This should give the community and maintainers a clear set of
guidelines to follow when choosing whether or not to add issues and PRs to a
given release.

### End (almost) of the Release Cycle
The developer call closest to two weeks before the scheduled release date will
be used to review any remaining PRs that should be pulled into the release. This
is the place to debate whether or not we should wait before cutting a release
and any other concerns. At the end of this meeting, if the release date has not
been pushed out, the first RC should be cut. Subsequent developer calls in
between this meeting and the release date should have some time set aside to see
if any bugs were found. Once the release date is reached, the final release can
be cut

## A Maintainer's Guide to Releasing Helm

So you're in charge of a new release for Helm? Cool. Here's what to do...

![TODO: Nothing](images/nothing.png)

Just kidding! :trollface:

All releases will be of the form vX.Y.Z where X is the major version number, Y
is the minor version number and Z is the patch release number. This project
strictly follows [semantic versioning](https://semver.org/) so following this
step is critical.

It is important to note that this document assumes that the git remote in your
repository that corresponds to "https://github.com/helm/helm" is named
"upstream". If yours is not (for example, if you've chosen to name it "origin"
or something similar instead), be sure to adjust the listed snippets for your
local environment accordingly. If you are not sure what your upstream remote is
named, use a command like `git remote -v` to find out.

If you don't have an upstream remote, you can add one easily using something
like:

```shell
git remote add upstream git@github.com:helm/helm.git
```

In this doc, we are going to reference a few environment variables as well,
which you may want to set for convenience. For major/minor releases, use the
following:

```shell
export RELEASE_NAME=vX.Y.0
export RELEASE_BRANCH_NAME="release-X.Y"
export RELEASE_CANDIDATE_NAME="$RELEASE_NAME-rc.1"
```

If you are creating a patch release, you may want to use the following instead:

```shell
export PREVIOUS_PATCH_RELEASE=vX.Y.Z
export RELEASE_NAME=vX.Y.Z+1
export RELEASE_BRANCH_NAME="release-X.Y"
export RELEASE_CANDIDATE_NAME="$RELEASE_NAME-rc.1"
```

We are also going to be adding security and verification of the release process by
hashing the binaries and providing signature files. We perform this using
[GitHub and GPG](https://help.github.com/en/articles/about-commit-signature-verification).
If you do not have GPG already setup you can follow these steps:
1. [Install GPG](https://gnupg.org/index.html)
2. [Generate GPG key](https://help.github.com/en/articles/generating-a-new-gpg-key)
3. [Add key to GitHub account](https://help.github.com/en/articles/adding-a-new-gpg-key-to-your-github-account)
4. [Set signing key in Git](https://help.github.com/en/articles/telling-git-about-your-signing-key)

Once you have a signing key you need to add it to the KEYS file at the root of
the repository. The instructions for adding it to the KEYS file are in the file.
If you have not done so already, you need to add your public key to the keyserver
network. If you use GnuPG you can follow the [instructions provided by Debian](https://debian-administration.org/article/451/Submitting_your_GPG_key_to_a_keyserver).

## 1. Create the Release Branch

### Major/Minor Releases

Major releases are for new feature additions and behavioral changes *that break
backwards compatibility*. Minor releases are for new feature additions that do
not break backwards compatibility. To create a major or minor release, start by
creating a `release-vX.Y.0` branch from master.

```shell
git fetch upstream
git checkout upstream/master
git checkout -b $RELEASE_BRANCH_NAME
```

This new branch is going to be the base for the release, which we are going to
iterate upon later.

### Patch releases

Patch releases are a few critical cherry-picked fixes to existing releases.
Start by creating a `release-vX.Y.Z` branch from the latest patch release.

```shell
git fetch upstream --tags
git checkout $PREVIOUS_PATCH_RELEASE
git checkout -b $RELEASE_BRANCH_NAME
```

From here, we can cherry-pick the commits we want to bring into the patch
release:

```shell
# get the commits ids we want to cherry-pick
git log --oneline
# cherry-pick the commits starting from the oldest one, without including merge commits
git cherry-pick -x <commit-id>
git cherry-pick -x <commit-id>
```

This new branch is going to be the base for the release, which we are going to
iterate upon later.

## 2. Change the Version Number in Git

When doing a minor release, make sure to update pkg/version/version.go with the
new release version.

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

```shell
git add .
git commit -m "bump version to $RELEASE_CANDIDATE_NAME"
```

This will update it for the $RELEASE_BRANCH_NAME only. You will also need to pull
this change into the master branch for when the next release is being created.

```shell
# get the last commit id i.e. commit to bump the version
git log --format="%H" -n 1

# create new branch off master
git checkout master
git checkout -b bump-version-<release_version>

# cherry pick the commit using id from first command
git cherry-pick -x <commit-id>

# commit the change
git push origin bump-version-<release-version>
```

## 3. Commit and Push the Release Branch

In order for others to start testing, we can now push the release branch
upstream and start the test process.

```shell
git push upstream $RELEASE_BRANCH_NAME
```

Make sure to check [helm on CircleCI](https://circleci.com/gh/helm/helm) and
make sure the release passed CI before proceeding.

If anyone is available, let others peer-review the branch before continuing to
ensure that all the proper changes have been made and all of the commits for the
release are there.

## 4. Create a Release Candidate

Now that the release branch is out and ready, it is time to start creating and
iterating on release candidates.

```shell
git tag --sign --annotate "${RELEASE_CANDIDATE_NAME}" --message "Helm release ${RELEASE_CANDIDATE_NAME}"
git push upstream $RELEASE_CANDIDATE_NAME
```

CircleCI will automatically create a tagged release image and client binary to
test with.

For testers, the process to start testing after CircleCI finishes building the
artifacts involves the following steps to grab the client:

linux/amd64, using /bin/bash:

```shell
wget https://get.helm.sh/helm-$RELEASE_CANDIDATE_NAME-linux-amd64.tar.gz
```

darwin/amd64, using Terminal.app:

```shell
wget https://get.helm.sh/helm-$RELEASE_CANDIDATE_NAME-darwin-amd64.tar.gz
```

windows/amd64, using PowerShell:

```shell
PS C:\> Invoke-WebRequest -Uri "https://get.helm.sh/helm-$RELEASE_CANDIDATE_NAME-windows-amd64.zip" -OutFile "helm-$ReleaseCandidateName-windows-amd64.zip"
```

Then, unpack and move the binary to somewhere on your $PATH, or move it
somewhere and add it to your $PATH (e.g. /usr/local/bin/helm for linux/macOS,
C:\Program Files\helm\helm.exe for Windows).

## 5. Iterate on Successive Release Candidates

Spend several days explicitly investing time and resources to try and break helm
in every possible way, documenting any findings pertinent to the release. This
time should be spent testing and finding ways in which the release might have
caused various features or upgrade environments to have issues, not coding.
During this time, the release is in code freeze, and any additional code changes
will be pushed out to the next release.

During this phase, the $RELEASE_BRANCH_NAME branch will keep evolving as you
will produce new release candidates. The frequency of new candidates is up to
the release manager: use your best judgement taking into account the severity of
reported issues, testers' availability, and the release deadline date. Generally
speaking, it is better to let a release roll over the deadline than to ship a
broken release.

Each time you'll want to produce a new release candidate, you will start by
adding commits to the branch by cherry-picking from master:

```shell
git cherry-pick -x <commit_id>
```

You will also want to update the release version number and the CHANGELOG as we
did in steps 2 and 3 as separate commits.

After that, tag it and notify users of the new release candidate:

```shell
export RELEASE_CANDIDATE_NAME="$RELEASE_NAME-rc.2"
git tag --sign --annotate "${RELEASE_CANDIDATE_NAME}" --message "Helm release ${RELEASE_CANDIDATE_NAME}"
git push upstream $RELEASE_CANDIDATE_NAME
```

From here on just repeat this process, continuously testing until you're happy with the release candidate.

## 6. Finalize the Release

When you're finally happy with the quality of a release candidate, you can move
on and create the real thing. Double-check one last time to make sure everything
is in order, then finally push the release tag.

```shell
git checkout $RELEASE_BRANCH_NAME
git tag --sign --annotate "${RELEASE_NAME}" --message "Helm release ${RELEASE_NAME}"
git push upstream $RELEASE_NAME
```

Verify that the release succeeded in CI. If not, you will need to fix the 
release and push the release again.

## 7. PGP Sign the downloads

While hashes provide a signature that the content of the downloads is what it
was generated, signed packages provide traceability of where the package came
from.

To do this, run the following `make` commands:

```shell
export VERSION="$RELEASE_NAME"
make clean
make fetch-dist
make sign
```

This will generate ascii armored signature files for each of the files pushed by CI.

All of the signature files need to be uploaded to the release on GitHub.

## 8. Write the Release Notes

We will auto-generate a changelog based on the commits that occurred during a
release cycle, but it is usually more beneficial to the end-user if the release
notes are hand-written by a human being/marketing team/dog.

If you're releasing a major/minor release, listing notable user-facing features
is usually sufficient. For patch releases, do the same, but make note of the
symptoms and who is affected.

An example release note for a minor release would look like this:

```markdown
## vX.Y.Z

Helm vX.Y.Z is a feature release. This release, we focused on <insert focal point>. Users are encouraged to upgrade for the best experience.

The community keeps growing, and we'd love to see you there!

- Join the discussion in [Kubernetes Slack](https://kubernetes.slack.com):
  - `#helm-users` for questions and just to hang out
  - `#helm-dev` for discussing PRs, code, and bugs
- Hang out at the Public Developer Call: Thursday, 9:30 Pacific via [Zoom](https://zoom.us/j/696660622)
- Test, debug, and contribute charts: [GitHub/helm/charts](https://github.com/helm/charts)

## Features and Changes

- Major
- features
- list
- here

## Installation and Upgrading

Download Helm X.Y. The common platform binaries are here:

- [MacOS amd64](https://get.helm.sh/helm-vX.Y.Z-darwin-amd64.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-darwin-amd64.tar.gz.sha256) / CHECKSUM_VAL)
- [Linux amd64](https://get.helm.sh/helm-vX.Y.Z-linux-amd64.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-linux-amd64.tar.gz.sha256) / CHECKSUM_VAL)
- [Linux arm](https://get.helm.sh/helm-vX.Y.Z-linux-arm.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-linux-arm.tar.gz.sha256) / CHECKSUM_VAL)
- [Linux arm64](https://get.helm.sh/helm-vX.Y.Z-linux-arm64.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-linux-arm64.tar.gz.sha256) / CHECKSUM_VAL)
- [Linux i386](https://get.helm.sh/helm-vX.Y.Z-linux-386.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-linux-386.tar.gz.sha256) / CHECKSUM_VAL)
- [Linux ppc64le](https://get.helm.sh/helm-vX.Y.Z-linux-ppc64le.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-linux-ppc64le.tar.gz.sha256) / CHECKSUM_VAL)
- [Linux s390x](https://get.helm.sh/helm-vX.Y.Z-linux-s390x.tar.gz) ([checksum](https://get.helm.sh/helm-vX.Y.Z-linux-s390x.tar.gz.sha256) / CHECKSUM_VAL)
- [Windows amd64](https://get.helm.sh/helm-vX.Y.Z-windows-amd64.zip) ([checksum](https://get.helm.sh/helm-vX.Y.Z-windows-amd64.zip.sha256) / CHECKSUM_VAL)

Once you have the client installed, upgrade Tiller with `helm init --upgrade`.

The [Quickstart Guide](https://docs.helm.sh/using_helm/#quickstart-guide) will get you going from there. For **upgrade instructions** or detailed installation notes, check the [install guide](https://docs.helm.sh/using_helm/#installing-helm). You can also use a [script to install](https://raw.githubusercontent.com/helm/helm/master/scripts/get) on any system with `bash`.

## What's Next

- vX.Y.Z+1 will contain only bug fixes.
- vX.Y+1.Z is the next feature release. This release will focus on ...

## Changelog

### Features
- ref(*): kubernetes v1.11 support efadbd88035654b2951f3958167afed014c46bc6 (Adam Reese)
- feat(helm): add $HELM_KEY_PASSPHRASE environment variable for signing helm charts (#4778) 1e26b5300b5166fabb90002535aacd2f9cc7d787

### Bug fixes
- fix circle not building tags f4f932fabd197f7e6d608c8672b33a483b4b76fa (Matthew Fisher)

### Code cleanup
- ref(kube): Gets rid of superfluous Sprintf call 3071a16f5eb3a2b646d9795617287cc26e53dba4  (Taylor Thomas)
- chore(*): bump version to v2.7.0 08c1144f5eb3e3b636d9775617287cc26e53dba4 (Adam Reese)

### Documentation Changes
- docs(release_checklist): fix changelog generation command (#4694) 8442851a5c566a01d9b4c69b368d64daa04f6a7f (Matthew Fisher)
```

The changelog at the bottom of the release notes can be generated with this
command:

```shell
PREVIOUS_RELEASE=vX.Y.Z
git log --no-merges --pretty=format:'- %s %H (%aN)' $PREVIOUS_RELEASE..$RELEASE_NAME
```

After generating the changelog, you will need to categorize the changes as shown
in the example above.

Once finished, go into GitHub and edit the release notes for the tagged release with the notes written here.

Remember to attach the ascii armored signatures generated in the previous step to the release notes.

It is now worth getting other people to take a look at the release notes before the release is published. Send
a request out to [#helm-dev](https://kubernetes.slack.com/messages/C51E88VDG) for review. It is always 
beneficial as it can be easy to miss something.

When you are ready to go, hit `publish`.

## 9. Evangelize

Congratulations! You're done. Go grab yourself a $DRINK_OF_CHOICE. You've earned
it.

After enjoying a nice $DRINK_OF_CHOICE, go forth and announce the glad tidings
of the new release in Slack and on Twitter.

Optionally, write a blog post about the new release and showcase some of the new
features on there!
