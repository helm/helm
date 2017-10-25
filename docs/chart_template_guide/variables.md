# Variables

With functions, pipelines, objects, and control structures under our belts, we can turn to one of the more basic ideas in many programming languages: variables. In templates, they are less frequently used. But we will see how to use them to simplify code, and to make better use of `with` and `range`.

In an earlier example, we saw that this code will fail:

```yaml
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  release: {{ .Release.Name }}
  {{- end }}
```

`Release.Name` is not inside of the scope that's restricted in the `with` block. One way to work around scoping issues is to assign objects to variables that can be accessed without respect to the present scope.

In Helm templates, a variable is a named reference to another object. It follows the form `$name`. Variables are assigned with a special assignment operator: `:=`. We can rewrite the above to use a variable for `Release.Name`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- $relname := .Release.Name -}}
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  release: {{ $relname }}
  {{- end }}
```

Notice that before we start the `with` block, we assign `$relname := .Release.Name`. Now inside of the `with` block, the `$relname` variable still points to the release name.

Running that will produce this:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: viable-badger-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "PIZZA"
  release: viable-badger
```

Variables are particularly useful in `range` loops. They can be used on list-like objects to capture both the index and the value:

```yaml
  toppings: |-
    {{- range $index, $topping := .Values.pizzaToppings }}
      {{ $index }}: {{ $topping }}
    {{- end }}

```

Note that `range` comes first, then the variables, then the assignment operator, then the list. This will assign the integer index (starting from zero) to `$index` and the value to `$topping`. Running it will produce:

```yaml
  toppings: |-
      0: mushrooms
      1: cheese
      2: peppers
      3: onions
```

For data structures that have both a key and a value, we can use `range` to get both. For example, we can loop through `.Values.favorite` like this:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- range $key, $val := .Values.favorite }}
  {{ $key }}: {{ $val | quote }}
  {{- end}}
```

Now on the first iteration, `$key` will be `drink` and `$val` will be `coffee`, and on the second, `$key` will be `food` and `$val` will be `pizza`. Running the above will generate this:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: eager-rabbit-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "pizza"
```

Variables are normally not "global". They are scoped to the block in which they are declared. Earlier, we assigned `$relname` in the top level of the template. That variable will be in scope for the entire template. But in our last example, `$key` and `$val` will only be in scope inside of the `{{range...}}{{end}}` block.

However, there is one variable that is always global - `$` - this
variable will always point to the root context.  This can be very
useful when you are looping in a range need to know the chart's release
name.

An example illustrating this:
```yaml
{{- range .Values.tlsSecrets }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ .name }}
  labels:
    # Many helm templates would use `.` below, but that will not work, 
    # however `$` will work here 
    app: {{ template "fullname" $ }}
    # I cannot reference .Chart.Name, but I can do $.Chart.Name
    chart: "{{ $.Chart.Name }}-{{ $.Chart.Version }}"
    release: "{{ $.Release.Name }}"
    heritage: "{{ $.Release.Service }}"
type: kubernetes.io/tls
data:
  tls.crt: {{ .certificate }}
  tls.key: {{ .key }}
---
{{- end }}
```

So far we have looked at just one template declared in just one file. But one of the powerful features of the Helm template language is its ability to declare multiple templates and use them together. We'll turn to that in the next section.
