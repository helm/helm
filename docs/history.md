## The History of the Project

Kubernetes Helm is the merged result of [Helm
Classic](https://github.com/helm/helm) and the Kubernetes port of GCS Deployment
Manager. The project was jointly started by Google and Deis, though it
is now part of the CNCF. Many companies now contribute regularly to Helm.

Differences from Helm Classic:

- Helm now has both a client (`helm`) and a server (`tiller`). The
  server runs inside of Kubernetes, and manages your resources.
- Helm's chart format has changed for the better:
  - Dependencies are immutable and stored inside of a chart's `charts/`
    directory.
  - Charts are strongly versioned using [SemVer 2](http://semver.org/spec/v2.0.0.html)
  - Charts can be loaded from directories or from chart archive files
  - Helm supports Go templates without requiring you to run `generate`
    or `template` commands.
  - Helm makes it easy to configure your releases -- and share the
    configuration with the rest of your team.
- Helm chart repositories now use plain HTTP(S) instead of Git/GitHub.
  There is no longer any GitHub dependency.
  - A chart server is a simple HTTP server
  - Charts are referenced by version
  - The `helm serve` command will run a local chart server, though you
    can easily use object storage (S3, GCS) or a regular web server.
  - And you can still load charts from a local directory.
- The Helm workspace is gone. You can now work anywhere on your
  filesystem that you want to work.
