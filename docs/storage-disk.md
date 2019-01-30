# Disk storage

If you have very large charts (above 1MB), using disk storage is a suggested solution. Etcd has a file size limit of 1MB which will cause deployments of very large charts to fail.

## Usage

You need to start tiller up to use the disk storage option

```shell
helm init --override 'spec.template.spec.containers[0].command'='{/tiller,--storage=disk}'
```

While this method will work, it's not recommended since it won't survive pod restarts, since the data is saved inside
the docker image.
The solution is to do a manually deploy of tiller with a volume mount.

A solution to this can be seen below. Please verify that the image to install is the correct version

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  labels:
    app: helm
    name: tiller
  name: tiller-deploy
  namespace: kube-system
spec:
  progressDeadlineSeconds: 600
  replicas: 1
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: helm
      name: tiller
  strategy:
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: helm
        name: tiller
    spec:
      automountServiceAccountToken: true
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: tiller-releases
      initContainers:
        - name: take-data-dir-ownership
          image: alpine:3.6
          command:
            - chown
            - -R
            - nobody:nobody
            - /releases
          volumeMounts:
            - name: data
              mountPath: /releases
      containers:
        - command:
            - /tiller
            - --storage=disk
          env:
            - name: TILLER_NAMESPACE
              value: kube-system
            - name: TILLER_HISTORY_MAX
              value: "0"
          image: "gcr.io/kubernetes-helm/tiller:v2.13"
          imagePullPolicy: Always
          volumeMounts:
            - name: data
              mountPath: /releases
          livenessProbe:
            failureThreshold: 3
            httpGet:
              path: /liveness
              port: 44135
              scheme: HTTP
            initialDelaySeconds: 1
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
          name: tiller
          ports:
            - containerPort: 44134
              name: tiller
              protocol: TCP
            - containerPort: 44135
              name: http
              protocol: TCP
          readinessProbe:
            failureThreshold: 3
            httpGet:
              path: /readiness
              port: 44135
              scheme: HTTP
            initialDelaySeconds: 1
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 1
      serviceAccount: tiller
      serviceAccountName: tiller
      terminationGracePeriodSeconds: 30
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    heritage: Tiller
  name: tiller-releases
  namespace: kube-system
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 8Gi
  storageClassName: default
```
