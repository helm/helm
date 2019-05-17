# Registries

Helm 3 uses <a href="https://www.opencontainers.org/" target="_blank">OCI</a> for package distribution. Chart packages are stored and shared across OCI-based registries.

## Running a registry

Starting a registry for test purposes is trivial. As long as you have Docker installed, run the following command:
```
docker run -dp 5000:5000 --restart=always --name registry registry
```

This will start a registry server at `localhost:5000`.

Use `docker logs -f registry` to see the logs and `docker rm -f registry` to stop.

If you wish to persist storage, you can add `-v $(pwd)/registry:/var/lib/registry` to the command above.

For more configuration options, please see [the docs](https://docs.docker.com/registry/deploying/).

### Auth

If you wish to enable auth on the registry, you can do the following-

First, create file `auth.htpasswd` with username and password combo:
```
htpasswd -cB -b auth.htpasswd myuser mypass
```

Then, start the server, mounting that file and setting the `REGISTRY_AUTH` env var:
```
docker run -dp 5000:5000 --restart=always --name registry \
  -v $(pwd)/auth.htpasswd:/etc/docker/registry/auth.htpasswd \
  -e REGISTRY_AUTH="{htpasswd: {realm: localhost, path: /etc/docker/registry/auth.htpasswd}}" \
  registry
```

## Commands for working with registries

Commands are available under both `helm registry` and `helm chart` that allow you to work with registries and local cache.

### The `registry` subcommand

#### `login`

login to a registry (with manual password entry)

```
$ helm registry login -u myuser localhost:5000
Password:
Login succeeded
```

#### `logout`

logout from a registry

```
$ helm registry logout localhost:5000
Logout succeeded
```

### The `chart` subcommand

#### `save`

save a chart directory to local cache

```
$ helm chart save mychart/ localhost:5000/myrepo/mychart:2.7.0
Name: mychart
Version: 2.7.0
Meta: sha256:ca9588a9340fb83a62777cd177dae4ba5ab52061a1618ce2e21930b86c412d9e
Content: sha256:a66666c6b35ee25aa8ecd7d0e871389b5a2a0576295d6c366aefe836001cb90d
2.7.0: saved
```

#### `list`

list all saved charts

```
$ helm chart list
REF                                                     NAME                    VERSION DIGEST  SIZE            CREATED
localhost:5000/myrepo/mychart:2.7.0                     mychart                 2.7.0   84059d7 454 B           27 seconds
localhost:5000/stable/acs-engine-autoscaler:2.2.2       acs-engine-autoscaler   2.2.2   d8d6762 4.3 KiB         2 hours
localhost:5000/stable/aerospike:0.2.1                   aerospike               0.2.1   4aff638 3.7 KiB         2 hours
localhost:5000/stable/airflow:0.13.0                    airflow                 0.13.0  c46cc43 28.1 KiB        2 hours
localhost:5000/stable/anchore-engine:0.10.0             anchore-engine          0.10.0  3f3dcd7 34.3 KiB        2 hours
...
```

#### `export`

export a chart to directory

```
$ helm chart export localhost:5000/myrepo/mychart:2.7.0
Name: mychart
Version: 2.7.0
Meta: sha256:3344059bb81c49cc6f2599a379da0a6c14313cf969f7b821aca18e489ba3991b
Content: sha256:84059d7403f496a1c63caf97fdc5e939ea39e561adbd98d0aa864d1b9fc9653f
Exported to mychart/
```

#### `push`

push a chart to remote

```
$ helm chart push localhost:5000/myrepo/mychart:2.7.0
The push refers to repository [localhost:5000/myrepo/mychart]
Name: mychart
Version: 2.7.0
Meta: sha256:ca9588a9340fb83a62777cd177dae4ba5ab52061a1618ce2e21930b86c412d9e
Content: sha256:a66666c6b35ee25aa8ecd7d0e871389b5a2a0576295d6c366aefe836001cb90d
2.7.0: pushed to remote (2 layers, 478 B total)
```

#### `remove`

remove a chart from cache

```
$ helm chart remove localhost:5000/myrepo/mychart:2.7.0
2.7.0: removed
```

#### `pull`

pull a chart from remote

```
$ helm chart pull localhost:5000/myrepo/mychart:2.7.0
2.7.0: Pulling from localhost:5000/myrepo/mychart
Name: mychart
Version: 2.7.0
Meta: sha256:ca9588a9340fb83a62777cd177dae4ba5ab52061a1618ce2e21930b86c412d9e
Content: sha256:a66666c6b35ee25aa8ecd7d0e871389b5a2a0576295d6c366aefe836001cb90d
Status: Chart is up to date for localhost:5000/myrepo/mychart:2.7.0
```

## Where are my charts?

Charts stored using the commands above will be cached on disk at `~/.helm/registry` (or somewhere else depending on `$HELM_HOME`).

Chart content (tarball) and chart metadata (json) are stored as separate content-addressable blobs.  This prevents storing the same content twice when, for example, you are simply modifying some fields in `Chart.yaml`. They are joined together and converted back into regular chart format when using the `export` command.

The chart name and chart version are treated as "first-class" properties and stored separately. They are extracted out of `Chart.yaml` prior to building the metadata blob.

The following shows an example of a single chart stored in the cache (`localhost:5000/myrepo/mychart:2.7.0`):
```
$ tree ~/.helm/registry
/Users/me/.helm/registry
├── blobs
│   └── sha256
│       ├── 3344059bb81c49cc6f2599a379da0a6c14313cf969f7b821aca18e489ba3991b
│       └── 84059d7403f496a1c63caf97fdc5e939ea39e561adbd98d0aa864d1b9fc9653f
├── charts
│   └── mychart
│       └── versions
│           └── 2.7.0
└── refs
    └── localhost_5000
        └── myrepo
            └── mychart
                └── tags
                    └── 2.7.0
                        ├── chart -> /Users/me/.helm/registry/charts/mychart/versions/2.7.0
                        ├── content -> /Users/me/.helm/registry/blobs/sha256/3344059bb81c49cc6f2599a379da0a6c14313cf969f7b821aca18e489ba3991b
                        └── meta -> /Users/me/.helm/registry/blobs/sha256/84059d7403f496a1c63caf97fdc5e939ea39e561adbd98d0aa864d1b9fc9653f
```

## Migrating from chart repos

Migrating from classic [chart repositories](./chart_repository.md) (index.yaml-based repos) is as simple as a `helm fetch` (Helm 2 CLI), `helm chart save`, `helm chart push`.
