# Chart Repositories: Frequently Asked Questions

This section tracks some of the more frequently encountered issues with using chart repositories.

**We'd love your help** making this document better. To add, correct, or remove
information, [file an issue](https://github.com/kubernetes/helm/issues) or
send us a pull request.

## Fetching

**Q: Why do I get a `unsupported protocol scheme ""` error when trying to fetch a chart from my custom repo?**

A: (Helm < 2.5.0) This is likely caused by you creating your chart repo index without specifying the `--url` flag.
Try recreating your `index.yaml` file with a command like `helm repo index --url http://my-repo/charts .`,
and then re-uploading it to your custom charts repo.

This behavior was changed in Helm 2.5.0.
