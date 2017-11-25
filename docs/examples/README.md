# Helm Examples

This directory contains example charts to help you get started with
chart development.

## Alpine

The `alpine` chart is very simple, and is a good starting point.

It simply deploys a single pod running Alpine Linux.

## Nginx

The `nginx` chart shows how to compose several resources into one chart,
and it illustrates more complex template usage.

It deploys a `deployment` (which creates a `replica set`), a `config
map`, and a `service`. The replica set starts an nginx pod. The config
map stores the files that the nginx server can serve.

## Guestbook

The `guestbook` chart is a simple multi-tier app. The first tier is Apache httpd-based app that uses Redis master-slave cluster (the second tier). 

The app comprises of `deployment` for the each tier that are accessible thru its corresponding `service`
