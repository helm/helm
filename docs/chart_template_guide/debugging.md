# Debugging Templates

Debugging templates can be tricky simply because the templates are rendered on the Tiller server, not the Helm client. And then the rendered templates are sent to the Kubernetes API server, which may reject the YAML files for reasons other than formatting.

There are a few commands that can help you debug.

- `helm lint` is your go-to tool for verifying that your chart follows best practices
- `helm install --dry-run --debug`: We've seen this trick already. It's a great way to have the server render your templates, then return the resulting manifest file.
- `helm get manifest`: This is a good way to see what templates are installed on the server.

When your YAML is failing to parse, but you want to see what is generated, one
easy way to retrieve the YAML is to comment out the problem section in the template,
and then re-run `helm install --dry-run --debug`:

```YAML
apiVersion: v1
# some: problem section
# {{ .Values.foo | quote }}
```

The above will be rendered and returned with the comments intact:

```YAML
apiVersion: v1
# some: problem section
#  "bar"
```

This provides a quick way of viewing the generated content without YAML parse
errors blocking.
