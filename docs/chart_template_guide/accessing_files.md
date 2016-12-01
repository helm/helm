# Accessing Files Inside Templates

In the previous section we looked at several ways to create and access named templates. This makes it easy to import one template from within another template. But sometimes it is desirable to import a _file that is not a template_ and inject its contents without sending the contents through the template renderer.

Helm provides access to files through the `.Files` object. Before we get going with the template examples, though, there are a few things to note about how this works:

- It is okay to add extra files to your Helm chart. These files will be bundled and set to Tiller. Be careful, though. Charts must be smaller than 1M because of the storage limitations of Kubernetes objects.
- Some files cannot be accessed through the `.Files` object, usually for security reasons.
	- Files in `templates/` cannot be accessed.
- Charts to not preserve UNIX mode information, so file-level permissions will have no impact on the availability of a file when it comes to the `.Files` object.

## Basic example

With those caveats behind, let's write a template that reads three files into our ConfigMap. To get started, we will add three files to the chart, putting all three directly inside of the `mychart/` directory.

`config1.toml`:

```toml
message = Hello from config 1
```

`config2.toml`:

```toml
message = This is config 2
```

`config3.toml`:

```toml
message = Goodbye from config 3
```

Each of these is a simple TOML file (think old-school Windows INI files). We know the names of these files, so we can use a `range` function to loop through them and inject their contents into our ConfigMap.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  {{- $files := .Files }}
  {{- range tuple "config1.toml" "config2.toml" "config3.toml" }}
  {{ . }}: |-
    {{ $files.Get . }}
  {{- end }}
```

This config map uses several of the techniques discussed in previous sections. For example, we create a `$files` variable to hold a reference to the `.Files` object. We also use the `tuple` function to create a list of files that we loop through. Then we print each file name (`{{.}}: |-`) followed by the contents of the file `{{ $files.Get . }}`.

Running this template will produce a single ConfigMap with the contents of all three files:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: quieting-giraf-configmap
data:
  config1.toml: |-
    message = Hello from config 1

  config2.toml: |-
    message = This is config 2

  config3.toml: |-
    message = Goodbye from config 3
```

## Glob patterns

As your chart grows, you may find you have a greater need to organize your
files more, and so we provide a `Files.Glob(pattern string)` method to assist
in extracting certain files however you need.

For example, imagine the directory structure:

```
foo/: 
  foo.txt foo.yaml

bar/:
  bar.go bar.conf baz.yaml
```

You have multiple options with Globs:


```yaml
{{ range $path := .Files.Glob "**.yaml" }}
{{ $path }}: |
{{ .Files.Get $path }}
{{ end }}
```

Or

```yaml
{{ range $path, $bytes := .Files.Glob "foo/*" }}
{{ $path }}: '{{ b64enc $bytes }}'
{{ end }}
```

## Secrets

When working with a Secret resource, you can import a file and have the template base-64 encode it for you:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-secret
type: Opaque
data:
  token: |-
    {{ .Files.Get "config1.toml" | b64enc }}
```

The above will take the same `config1.toml` file we used before and encode it:

```yaml
# Source: mychart/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: lucky-turkey-secret
type: Opaque
data:
  token: |-
    bWVzc2FnZSA9IEhlbGxvIGZyb20gY29uZmlnIDEK
```

Currently, there is no way to pass files external to the chart during `helm install`. So if you are asking users to supply data, it must be loaded using `helm install -f` or `helm install --set`.

This discussion wraps up our dive into the tools and techniques for writing Helm templates. In the next section we will see how you can use one special file, `templates/NOTES.txt`, to send post-installation instructions to the users of your chart.

