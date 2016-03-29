# Pushing Charts

This details the current requirements and steps for pushing a chart to
Google Cloud Storage (GCS) using gsutil tool.

## Prerequisites

In order to create and push a Chart, you must:
* have a bucket in GCS with write permissions
* have [gsutil](https://cloud.google.com/storage/docs/gsutil) tool configured and installed

## Creating a chart (optional)

If you already have a [chart](./design/chart_format.md) in the file system format, you can
skip this step.
./bin/helm chart create mychart

This will create the following directory structure:
vaikas@vaikas-glaptop:~/projects/dmos/src/github.com/kubernetes/helm$ find mychart
mychart
mychart/Chart.yaml
mychart/hooks
mychart/templates
mychart/docs

You can then create your own chart (examples/charts has examples to get you started)

## Serializing the chart

Helm tool can package a chart for you in the correct format and with name that
matches the version in the Chart.yaml file.

```
./bin/helm chart package <yourchart>
```

Using one of the examples [nginx](examples/charts/nginx/Chart.yaml)

```
./bin/helm chart package examples/charts/nginx
```

The resulting file will be nginx-0.0.1.tgz

## Pushing the chart

Using gsutil you can copy this file to your bucket with the following command:
```
gsutil cp <yourchart> gs://<yourbucket>/
```
or if you want to make this chart publicly readable:
```
gsutil cp -a public-read <yourchart> gs://<yourbucket>/
```

To handle more granular permissions, use the 'gsutil help acls' command.

Using the example above, you could make this publicy readable by doing:
```
gsutil cp -a public-read  nginx-0.0.1.tgz gs://<yourbucket>/
```



