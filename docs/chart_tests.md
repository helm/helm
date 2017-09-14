# Chart Tests

A chart contains a number of Kubernetes resources and components that work together. As a chart author, you may want to write some tests that validate that your chart works as expected when it is installed. These tests also help the chart consumer understand what your chart is supposed to do.

A **test** in a helm chart lives under the `templates/` directory and is a pod definition that specifies a container with a given command to run. The container should exit successfully (exit 0) for a test to be considered a success. The pod definition must contain one of the helm test hook annotations: `helm.sh/hooks: test-success` or `helm.sh/hooks: test-failure`.

Example tests:
- Validate that your configuration from the values.yaml file was properly injected.
  - Make sure your username and password work correctly
  - Make sure an incorrect username and password does not work
- Assert that your services are up and correctly load balancing
- etc.

You can run the pre-defined tests in Helm on a release using the command `helm test <RELEASE_NAME>`. For a chart consumer, this is a great way to sanity check that their release of a chart (or application) works as expected.

## A Breakdown of the Helm Test Hooks

In Helm, there are two test hooks: `test-success` and `test-failure`

`test-success` indicates that test pod should complete successfully. In other words, the containers in the pod should exit 0.
`test-failure` is a way to assert that a test pod should not complete successfully. If the containers in the pod do not exit 0, that indicates success.

## Example Test

Here is an example of a helm test pod definition in an example mariadb chart:

```
mariadb/
  Chart.yaml
  README.md
  values.yaml
  charts/
  templates/
  templates/tests/test-mariadb-connection.yaml
```
In `wordpress/templates/tests/test-mariadb-connection.yaml`:
```
apiVersion: v1
kind: Pod
metadata:
  name: "{{ .Release.Name }}-credentials-test"
  annotations:
    "helm.sh/hook": test-success
spec:
  containers:
  - name: {{ .Release.Name }}-credentials-test
    image: {{ .Values.image }}
    env:
      - name: MARIADB_HOST
        value: {{ template "mariadb.fullname" . }}
      - name: MARIADB_PORT
        value: "3306"
      - name: WORDPRESS_DATABASE_NAME
        value: {{ default "" .Values.mariadb.mariadbDatabase | quote }}
      - name: WORDPRESS_DATABASE_USER
        value: {{ default "" .Values.mariadb.mariadbUser | quote }}
      - name: WORDPRESS_DATABASE_PASSWORD
        valueFrom:
          secretKeyRef:
            name: {{ template "mariadb.fullname" . }}
            key: mariadb-password
    command: ["sh", "-c", "mysql --host=$MARIADB_HOST --port=$MARIADB_PORT --user=$WORDPRESS_DATABASE_USER --password=$WORDPRESS_DATABASE_PASSWORD"]
  restartPolicy: Never
```

## Steps to Run a Test Suite on a Release
1. `$ helm install mariadb`
```
NAME:   quirky-walrus
LAST DEPLOYED: Mon Feb 13 13:50:43 2017
NAMESPACE: default
STATUS: DEPLOYED
```

2. `$ helm test quirky-walrus`
```
RUNNING: quirky-walrus-credentials-test
SUCCESS: quirky-walrus-credentials-test
```

## Notes
- You can define as many tests as you would like in a single yaml file or spread across several yaml files in the `templates/` directory
- You are welcome to nest your test suite under a `tests/` directory like `<chart-name>/templates/tests/` for more isolation
