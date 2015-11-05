# Guestbook Example

Guestbook example shows how to bring up the
[Guestbook Example](https://github.com/kubernetes/kubernetes/tree/master/examples/guestbook)
from Kubernetes using Deployment Manager. It also shows you how to construct
and reuse parameterized templates. 

## Getting started

It is assumed that you have bootstrapped the Deployment Manager on your cluster
by following the [README.md][https://github.com/kubernetes/deployment-manager/blob/master/README.md]
for bootstrapping the cluster. 

## Deploying Guestbook
To deploy the Guestbook example, you run the following command.

```
client --name guestbook --service=http://localhost:8001/api/v1/proxy/namespaces/default/services/manager-service:manager examples/guestbook/guestbook.yaml
```

### Replicated Service

Typical pattern for deploying microservices in Kubernetes is to create both a
Replication Controller and a Service. We have created a parameterizable type
for that called [Replicated Service](https://github.com/kubernetes/deployment-manager/tree/master/examples/replicatedservice)
that we use throughout this example.

The Guestbook example consists of 2 services, a frontend and a Redis service.
Frontend is a replicated service with 3 replicas and is created like so:
```
- name: frontend
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/examples/replicatedservice/replicatedservice.py
  properties:
    service_port: 80
    container_port: 80
    external_service: true
    replicas: 3
    image: gcr.io/google_containers/example-guestbook-php-redis:v3
```

Redis is a composite type and consists of two replicated services. A master with a single replica
and the slaves with 2 replicas. It's construced as follows:
```
{% set REDIS_PORT = 6379 %}
{% set WORKERS = properties['workers'] or 2 %}

resources:
- name: redis-master
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/examples/replicatedservice/replicatedservice.py
  properties:
    # This has to be overwritten since service names are hard coded in the code
    service_name: redis-master
    service_port: {{ REDIS_PORT }}
    target_port: {{ REDIS_PORT }}
    container_port: {{ REDIS_PORT }}
    replicas: 1
    container_name: master
    image: redis

- name: redis-slave
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/examples/replicatedservice/replicatedservice.py
  properties:
    # This has to be overwritten since service names are hard coded in the code
    service_name: redis-slave
    service_port: {{ REDIS_PORT }}
    container_port: {{ REDIS_PORT }}
    replicas: {{ WORKERS }}
    container_name: worker
    image: kubernetes/redis-slave:v2
    # An example of how to specify env variables.
    env:
    - name: GET_HOSTS_FROM
      value: env
    - name: REDIS_MASTER_SERVICE_HOST
      value: redis-master
```

### Displaying types

You can also see what types have been deployed to the cluster:
```
client --action listtypes --service=http://localhost:8001/api/v1/proxy/namespaces/default/services/manager-service:manager 

["Service","ReplicationController","redis.jinja","https://raw.githubusercontent.com/kubernetes/deployment-manager/master/examples/replicatedservice/replicatedservice.py"]
```

This shows that there are 2 native types that we have deployed (Service and ReplicationController) and
2 composite types (redis.jinja and one imported from github (replicatedservice.py)).


You can also see where the types are being used by getting details on the particular type:
```
client -action gettype --service=http://localhost:8001/api/v1/proxy/namespaces/default/services/manager-service:manager -name 'Service'
[{"name":"frontend-service","type":"Service","deployment":"guestbook4","manifest":"manifest-1446682551242763329","path":"$.resources[0].resources[0]"},{"name":"redis-master","type":"Service","deployment":"guestbook4","manifest":"manifest-1446682551242763329","path":"$.resources[1].resources[0].resources[0]"},{"name":"redis-slave","type":"Service","deployment":"guestbook4","manifest":"manifest-1446682551242763329","path":"$.resources[1].resources[1].resources[0]"}]
```

It lists which deployment and manifest as well as JSON path to the type.

