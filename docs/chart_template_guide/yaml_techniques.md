# YAML Techniques

Most of this guide has been focused on writing the template language. Here,
we'll look at the YAML format. YAML has some useful features that we, as
template authors, can use to make our templates less error prone and easier
to read.

## Scalars and Collections

According to the [YAML spec](http://yaml.org/spec/1.2/spec.html), there are two
types of collections, and many scalar types.

The two types of collections are maps and sequences:

```yaml
map:
  one: 1
  two: 2
  three: 3

sequence:
  - one
  - two
  - three
```

Scalar values are individual values (as opposed to collections)

### Scalar Types in YAML

In Helm's dialect of YAML, the scalar data type of a value is determined by a
complex set of rules, including the Kubernetes schema for resource definitions.
But when inferring types, the following rules tend to hold true.

If an integer or float is an unquoted bare word, it is typically treated as
a numeric type:

```yaml
count: 1
size: 2.34
```

But if they are quoted, they are treated as strings:

```yaml
count: "1" # <-- string, not int
size: '2.34' # <-- string, not float
```

The same is true of booleans:

```yaml
isGood: true   # bool
answer: "true" # string
```

The word for an empty value is `null` (not `nil`).

Note that `port: "80"` is valid YAML, and will pass through both the
template engine and the YAML parser, but will fail if Kubernetes expects
`port` to be an integer.

In some cases, you can force a particular type inference using YAML node tags:

```yaml
coffee: "yes, please"
age: !!str 21
port: !!int "80"
```

In the above, `!!str` tells the parser that `age` is a string, even if it looks
like an int. And `port` is treated as an int, even though it is quoted.


## Strings in YAML

Much of the data that we place in YAML documents are strings. YAML has more than
one way to represent a string. This section explains the ways and demonstrates
how to use some of them.

There are three "inline" ways of declaring a string:

```yaml
way1: bare words
way2: "double-quoted strings"
way3: 'single-quoted strings'
```

All inline styles must be on one line.

- Bare words are unquoted, and are not escaped. For this reason, you have to
  be careful what characters you use.
- Double-quoted strings can have specific characters escaped with `\`. For
  example `"\"Hello\", she said"`. You can escape line breaks with `\n`.
- Single-quoted strings are "literal" strings, and do not use the `\` to
  escape characters. The only escape sequence is `''`, which is decoded as
  a single `'`.

In addition to the one-line strings, you can declare multi-line strings:

```yaml
coffee: |
  Latte
  Cappuccino
  Espresso
```

The above will treat the value of `coffee` as a single string equivalent to
`Latte\nCappuccino\nEspresso\n`.

Note that the first line after the `|` must be correctly indented. So we could
break the example above by doing this:

```yaml
coffee: |
         Latte
  Cappuccino
  Espresso

```

Because `Latte` is incorrectly indented, we'd get an error like this:

```
Error parsing file: error converting YAML to JSON: yaml: line 7: did not find expected key
```

In templates, it is sometimes safer to put a fake "first line" of content in a
multi-line document just for protection from the above error:

```yaml
coffee: |
  # Commented first line
         Latte
  Cappuccino
  Espresso

```

Note that whatever that first line is, it will be preserved in the output of the
string. So if you are, for example, using this technique to inject a file's contents
into a ConfigMap, the comment should be of the type expected by whatever is
reading that entry.

### Controlling Spaces in Multi-line Strings

In the example above, we used `|` to indicate a multi-line string. But notice
that the content of our string was followed with a trailing `\n`. If we want
the YAML processor to strip off the trailing newline, we can add a `-` after the
`|`:

```yaml
coffee: |-
  Latte
  Cappuccino
  Espresso
```

Now the `coffee` value will be: `Latte\nCappuccino\nEspresso` (with no trailing
`\n`).

Other times, we might want all trailing whitespace to be preserved. We can do
this with the `|+` notation:

```yaml
coffee: |+
  Latte
  Cappuccino
  Espresso


another: value
```

Now the value of `coffee` will be `Latte\nCappuccino\nEspresso\n\n\n`.

Indentation inside of a text block is preserved, and results in the preservation
of line breaks, too:

```
coffee: |-
  Latte
    12 oz
    16 oz
  Cappuccino
  Espresso
```

In the above case, `coffee` will be `Latte\n  12 oz\n  16 oz\nCappuccino\nEspresso`.

### Indenting and Templates

When writing templates, you may find yourself wanting to inject the contents of
a file into the template. As we saw in previous chapters, there are two ways
of doing this:

- Use `{{ .Files.Get "FILENAME" }}` to get the contents of a file in the chart.
- Use `{{ include "TEMPLATE" . }}` to render a template and then place its
  contents into the chart.

When inserting files into YAML, it's good to understand the multi-line rules above.
Often times, the easiest way to insert a static file is to do something like
this:

```yaml
myfile: |
{{ .Files.Get "myfile.txt" | indent 2 }}
```

Note how we do the indentation above: `indent 2` tells the template engine to
indent every line in "myfile.txt" with two spaces. Note that we do not indent
that template line. That's because if we did, the file content of the first line
would be indented twice.

### Folded Multi-line Strings

Sometimes you want to represent a string in your YAML with multiple lines, but
want it to be treated as one long line when it is interpreted. This is called
"folding". To declare a folded block, use `>` instead of `|`:

```yaml
coffee: >
  Latte
  Cappuccino
  Espresso


```

The value of `coffee` above will be `Latte Cappuccino Espresso\n`. Note that all
but the last line feed will be converted to spaces. You can combine the whitespace
controls with the folded text marker, so `>-` will replace or trim all newlines.

Note that in the folded syntax, indenting text will cause lines to be preserved.

```yaml
coffee: >-
  Latte
    12 oz
    16 oz
  Cappuccino
  Espresso
```

The above will produce `Latte\n  12 oz\n  16 oz\nCappuccino Espresso`. Note that
both the spacing and the newlines are still there.

## Embedding Multiple Documents in One File

It is possible to place more than one YAML documents into a single file. This
is done by prefixing a new document with `---` and ending the document with
`...`

```yaml

---
document:1
...
---
document: 2
...
```

In many cases, either the `---` or the `...` may be omitted.

Some files in Helm cannot contain more than one doc. If, for example, more
than one document is provided inside of a `values.yaml` file, only the first
will be used.

Template files, however, may have more than one document. When this happens,
the file (and all of its documents) is treated as one object during
template rendering. But then the resulting YAML is split into multiple
documents before it is fed to Kubernetes.

We recommend only using multiple documents per file when it is absolutely
necessary. Having multiple documents in a file can be difficult to debug.

## YAML is a Superset of JSON

Because YAML is a superset of JSON, any valid JSON document _should_ be valid
YAML.

```json
{
  "coffee": "yes, please",
  "coffees": [
    "Latte", "Cappuccino", "Espresso"
  ]
}
```

The above is another way of representing this:

```yaml
coffee: yes, please
coffees:
- Latte
- Cappuccino
- Espresso
```

And the two can be mixed (with care):

```yaml
coffee: "yes, please"
coffees: [ "Latte", "Cappuccino", "Espresso"]
```

All three of these should parse into the same internal representation.

While this means that files such as `values.yaml` may contain JSON data, Helm
does not treat the file extension `.json` as a valid suffix.

## YAML Anchors

The YAML spec provides a way to store a reference to a value, and later
refer to that value by reference. YAML refers to this as "anchoring":

```yaml
coffee: "yes, please"
favorite: &favoriteCoffee "Cappucino"
coffees:
  - Latte
  - *favoriteCoffee
  - Espresso
```

In the above, `&favoriteCoffee` sets a reference to `Cappuccino`. Later, that
reference is used as `*favoriteCoffee`. So `coffees` becomes
`Latte, Cappuccino, Espresso`.

While there are a few cases where anchors are useful, there is one aspect of
them that can cause subtle bugs: The first time the YAML is consumed, the
reference is expanded and then discarded.

So if we were to decode and then re-encode the example above, the resulting
YAML would be:

```YAML
coffee: yes, please
favorite: Cappucino
coffees:
- Latte
- Cappucino
- Espresso
```

Because Helm and Kubernetes often read, modify, and then rewrite YAML files,
the anchors will be lost.
