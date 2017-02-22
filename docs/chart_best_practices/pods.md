# Pods and PodTemplates

This part of the Best Practices Guide discusses formatting the Pod and PodTemplate
portions in chart manifests.

The following (non-exhaustive) list of resources use PodTemplates:

- Deployment
- ReplicationController
- ReplicaSet
- DaemonSet
- StatefulSet

## Images

A container image should use a fixed tag or the SHA of the image. It should not use the tags `latest`, `head`, `canary`, or other tags that are designed to be "floating".


Images _may_ be defined in the `values.yaml` file to make it easy to swap out images.

```
image: {{ .Values.redisImage | quote }}
```

An image and a tag _may_ be defined in `values.yaml` as two separate fields:

```
image: "{{ .Values.redisImage }}:{{ .Values.redisTag }}"
```

## ImagePullPolicy

The `imagePullPolicy` should default to an empty value, but allow users to override it:

```yaml
imagePullPolicy: {{ default "" .Values.imagePullPolicy | quote }}
```

## PodTemplates Should Declare Selectors

All PodTemplate sections should specify a selector. For example:

```yaml
selector:
  matchLabels:
      app: MyName
template:
  metadata:
    labels:
      app: MyName
```

This is a good practice because it makes the relationship between the set and
the pod.

But this is even more important for sets like Deployment.
Without this, the _entire_ set of labels is used to select matching pods, and
this will break if you use labels that change, like version or release date.


