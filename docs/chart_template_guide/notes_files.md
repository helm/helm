# Creating a NOTES.txt File

In this section we are going to look at Helm's tool for providing instructions to your chart users. At the end of a `chart install` or `chart upgrade`, Helm can print out a block of helpful information for users. This information is highly customizable using templates.

To add installation notes to your chart, simply create a `templates/NOTES.txt` file. This file is plain text, but it is processed like as a template, and has all the normal template functions and objects available.

Let's create a simple `NOTES.txt` file:

```
Thank you for installing {{ .Chart.Name }}.

Your release is named {{ .Release.Name }}.

To learn more about the release, try:

  $ helm status {{ .Release.Name }}
  $ helm get {{ .Release.Name }}

```

Now if we run `helm install ./mychart` we will see this message at the bottom:

```
RESOURCES:
==> v1/Secret
NAME                   TYPE      DATA      AGE
rude-cardinal-secret   Opaque    1         0s

==> v1/ConfigMap
NAME                      DATA      AGE
rude-cardinal-configmap   3         0s


NOTES:
Thank you for installing mychart.

Your release is named rude-cardinal.

To learn more about the release, try:

  $ helm status rude-cardinal
  $ helm get rude-cardinal
```

Using `NOTES.txt` this way is a great way to give your users detailed information about how to use their newly installed chart. Creating a `NOTES.txt` file is strongly recommended, though it is not required.
