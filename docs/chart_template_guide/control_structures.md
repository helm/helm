# Flow Control

Control structures (called "actions" in template parlance) provide you, the template author, with the ability to control the flow of a template's generation. Helm's template language provides the following control structures:

- `if`/`else` for creating conditional blocks
- `with` to specify a scope
- `range`, which provides a "for each"-style loop

In addition to these, it provides a few actions for declaring and using named template segments:

- `define` declares a new named template inside of your template
- `template` imports a named template
- `block` declares a special kind of fillable template area

In this section, we'll talk about `if`, `with`, and `range`. The others are covered in the "Named Templates" section later in this guide.

## If/Else

The first control structure we'll look at is for conditionally including blocks of text in a template. This is the `if`/`else` block.

The basic structure for a conditional looks like this:

```
{{ if PIPELINE }}
  # Do something
{{ else if OTHER PIPELINE }}
  # Do something else
{{ else }}
  # Default case
{{ end }}
```

Notice that we're now talking about _pipelines_ instead of values. The reason for this is to make it clear that control structures can execute an entire pipeline, not just evaluate a value.

A pipeline is evaluated as _false_ if the value is:

- a boolean false
- a numeric zero
- an empty string
- a `nil` (empty or null)
- an empty collection (`map`, `slice`, `tuple`, `dict`, `array`)

Under all other conditions, the condition is true.

Let's add a simple conditional to our ConfigMap. We'll add another setting if the drink is set to coffee:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}
  {{ if eq .Values.favorite.drink "coffee" }}mug: true{{ end }}
```

Since we commented out `drink: coffee` in our last example, the output should not include a `mug: true` flag. But if we add that line back into our `values.yaml` file, the output should look like this:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: eyewitness-elk-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "PIZZA"
  mug: true
```

## Controlling Whitespace

While we're looking at conditionals, we should take a quick look at the way whitespace is controlled in templates. Let's take the previous example and format it to be a little easier to read:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}
  {{if eq .Values.favorite.drink "coffee"}}
    mug: true
  {{end}}
```

Initially, this looks good. But if we run it through the template engine, we'll get an unfortunate result:

```console
$ helm install --dry-run --debug ./mychart
SERVER: "localhost:44134"
CHART PATH: /Users/mattbutcher/Code/Go/src/k8s.io/helm/_scratch/mychart
Error: YAML parse error on mychart/templates/configmap.yaml: error converting YAML to JSON: yaml: line 9: did not find expected key
```

What happened? We generated incorrect YAML because of the whitespacing above. 

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: eyewitness-elk-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "PIZZA"
    mug: true
```

`mug` is incorrectly indented. Let's simply out-dent that one line, and re-run:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}
  {{if eq .Values.favorite.drink "coffee"}}
  mug: true
  {{end}}
```

When we sent that, we'll get YAML that is valid, but still looks a little funny:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: telling-chimp-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "PIZZA"

  mug: true

```

Notice that we received a few empty lines in our YAML. Why? When the template engine runs, it _removes_ the contents inside of `{{` and `}}`, but it leaves the remaining whitespace exactly as is.

YAML ascribes meaning to whitespace, so managing the whitespace becomes pretty important. Fortunately, Helm templates have a few tools to help.

First, the curly brace syntax of template declarations can be modified with special characters to tell the template engine to chomp whitespace. `{{- ` (with the dash and space added) indicates that whitespace should be chomped left, while ` -}}` means whitespace to the right should be consumed. _Be careful! Newlines are whitespace!_

> Make sure there is a space between the `-` and the rest of your directive. `{{- 3 }}` means "trim left whitespace and print 3" while `{{-3}}` means "print -3".

Using this syntax, we can modify our template to get rid of those new lines:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}
  {{- if eq .Values.favorite.drink "coffee"}}
  mug: true
  {{- end}}
```

Just for the sake of making this point clear, let's adjust the above, and substitute an `*` for each whitespace that will be deleted following this rule. an `*` at the end of the line indicates a newline character that would be removed

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  drink: {{ .Values.favorite.drink | default "tea" | quote }}
  food: {{ .Values.favorite.food | upper | quote }}*
**{{- if eq .Values.favorite.drink "coffee"}}
  mug: true*
**{{- end}}

```

Keeping that in mind, we can run our template through Helm and see the result:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: clunky-cat-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "PIZZA"
  mug: true
```

Be careful with the chomping modifiers. It is easy to accidentally do things like this:

```yaml
  food: {{ .Values.favorite.food | upper | quote }}
  {{- if eq .Values.favorite.drink "coffee" -}}
  mug: true
  {{- end -}}

```

That will produce `food: "PIZZA"mug:true` because it consumed newlines on both sides.

> For the details on whitespace control in templates, see the [Official Go template documentation](https://godoc.org/text/template)

Finally, sometimes it's easier to tell the template system how to indent for you instead of trying to master the spacing of template directives. For that reason, you may sometimes find it useful to use the `indent` function (`{{indent 2 "mug:true"}}`).

## Modifying scope using `with`

The next control structure to look at is the `with` action. This controls variable scoping. Recall that `.` is a reference to _the current scope_. So `.Values` tells the template to find the `Values` object in the current scope.

The syntax for `with` is similar to a simple `if` statement:

```
{{ with PIPELINE }}
  # restricted scope
{{ end }}
```

Scopes can be changed. `with` can allow you to set the current scope (`.`) to a particular object. For example, we've been working with `.Values.favorites`. Let's rewrite our ConfigMap to alter the `.` scope to point to `.Values.favorites`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  {{- end }}
```

(Note that we removed the `if` conditional from the previous exercise)

Notice that now we can reference `.drink` and `.food` without qualifying them. That is because the `with` statement sets `.` to point to `.Values.favorite`. The `.` is reset to its previous scope after `{{ end }}`.

But here's a note of caution! Inside of the restricted scope, you will not be able to access the other objects from the parent scope. This, for example, will fail:

```yaml
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  release: {{ .Release.Name }}
  {{- end }}
```

It will produce an error because `Release.Name` is not inside of the restricted scope for `.`. However, if we swap the last two lines, all will work as expected because the scope is reset after `{{end}}`.

```yaml
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  {{- end }}
  release: {{ .Release.Name }}
```

After looking a `range`, we will take a look at template variables, which offer one solution to the scoping issue above.

## Looping with the `range` action

Many programming languages have support for looping using `for` loops, `foreach` loops, or similar functional mechanisms. In Helm's template language, the way to iterate through a collection is to use the `range` operator.

To start, let's add a list of pizza toppings to our `values.yaml` file:

```yaml
favorite:
  drink: coffee
  food: pizza
pizzaToppings:
  - mushrooms
  - cheese
  - peppers
  - onions
```

Now we have a list (called a `slice` in templates) of `pizzaToppings`. We can modify our template to print this list into our ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-configmap
data:
  myvalue: "Hello World"
  {{- with .Values.favorite }}
  drink: {{ .drink | default "tea" | quote }}
  food: {{ .food | upper | quote }}
  {{- end }}
  toppings: |-
    {{- range .Values.pizzaToppings }}
    - {{ . | title | quote }}
    {{- end }}

```

Let's take a closer look at the `toppings:` list. The `range` function will "range over" (iterate through) the `pizzaToppings` list. But now something interesting happens. Just like `with` sets the scope of `.`, so does a `range` operator. Each time through the loop, `.` is set to the current pizza topping. That is, the first time, `.` is set to `mushrooms`. The second iteration it is set to `cheese`, and so on.

We can send the value of `.` directly down a pipeline, so when we do `{{ . | title | quote }}`, it sends `.` to `title` (title case function) and then to `quote`. If we run this template, the output will be:

```yaml
# Source: mychart/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: edgy-dragonfly-configmap
data:
  myvalue: "Hello World"
  drink: "coffee"
  food: "PIZZA"
  toppings: |-
    - "Mushrooms"
    - "Cheese"
    - "Peppers"
    - "Onions"
```

Now, in this example we've done something tricky. The `toppings: |-` line is declaring a multi-line string. So our list of toppings is actually not a YAML list. It's a big string. Why would we do this? Because the data in ConfigMaps `data` is composed of key/value pairs, where both the key and the value are simple strings. To understand why this is the case, take a look at the [Kubernetes ConfigMap docs](http://kubernetes.io/docs/user-guide/configmap/). For us, though, this detail doesn't matter much.

> The `|-` marker in YAML takes a multi-line string. This can be a useful technique for embedding big blocks of data inside of your manifests, as exemplified here.

Sometimes it's useful to be able to quickly make a list inside of your template, and then iterate over that list. Helm templates have a function to make this easy: `tuple`. In computer science, a tuple is a list-like collection of fixed size, but with arbitrary data types. This roughly conveys the way a `tuple` is used.

```yaml
  sizes: |-
    {{- range tuple "small" "medium" "large" }}
    - {{ . }}
    {{- end }}
```

The above will produce this:

```yaml
  sizes: |-
    - small
    - medium
    - large
```

In addition to lists and tuples, `range` can be used to iterate over collections that have a key and a value (like a `map` or `dict`). We'll see how to do that in the next section when we introduce template variables.
