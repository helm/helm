# The .helmignore file

The `.helmignore` file is used to specify files you don't want to include in your helm chart.

If this file exists, the `helm package` command will ignore all the files that match the pattern specified in the `.helmignore` file while packaging your application.

This can help in avoiding unncessary or sensitive files or directories from being added in your helm chart.

The `.helmignore` file supports Unix shell glob matching, relative path matching, and negation (prefixed with !). Only one pattern per line is considered.

Here is an example `.helmignore` file:

```
# comment
.git
*/temp*
*/*/temp*
temp?
```

**We'd love your help** making this document better. To add, correct, or remove
information, [file an issue](https://github.com/helm/helm/issues) or
send us a pull request.
