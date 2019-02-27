# Store release information in an SQL database

You may be willing to store release information in an SQL database - in
particular, if your releases weigh more than 1MB and therefore [can't be stored in ConfigMaps or Secrets](https://github.com/helm/helm/issues/1413).

We recommend using [PostgreSQL](https://www.postgresql.org/).

This document describes how to deploy `postgres` atop Kubernetes. This being
said, using an out-of-cluster (managed or not) PostreSQL instance is totally
possible as well.

Here's a Kubernetes manifest you can apply to get a minimal PostreSQL pod
running on your Kubernetes cluster. **Don't forget to change the credentials
and, optionally, enable TLS in production deployments**.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: tiller-postgres
  namespace: kube-system
spec:
  ports:
    - port: 5432
  selector:
    app: helm
    name: postgres
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: tiller-postgres
  namespace: kube-system
spec:
  serviceName: tiller-postgres
  selector:
    matchLabels:
      app: helm
      name: postgres
  replicas: 1
  template:
    metadata:
      labels:
        app: helm
        name: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:11-alpine
          imagePullPolicy: Always
          ports:
            - containerPort: 5432
          env:
          - name: POSTGRES_DB
            value: helm
          - name: POSTGRES_USER
            value: helm
          - name: POSTGRES_PASSWORD
            value: changemeforgodssake
          - name: PGDATA
            value: /var/lib/postgresql/data/pgdata
          resources:
            limits:
              memory: 128Mi
            requests:
              cpu: 50m
              memory: 128Mi
          volumeMounts:
            - mountPath: /var/lib/postgresql/data
              name: tiller-postgres-data
  volumeClaimTemplates:
  - metadata:
      name: tiller-postgres-data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: default
      resources:
        requests:
          storage: 5Gi
```

Once postgres is deployed, you'll need to install Tiller using `helm init`, with
a few custom CLI flags:

```shell
helm init \
  --override \
    'spec.template.spec.containers[0].args'='{--storage=sql,--sql-dialect=postgres,--sql-connection-string=postgresql://tiller-postgres:5432/helm?user=helm&password=changemeforgodssake&sslmode=disable}'
```
