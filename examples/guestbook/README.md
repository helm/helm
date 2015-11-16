# Guestbook Example

Welcome to the Guestbook example. It shows you how to build and reuse
parameterized templates.

## Prerequisites

First, make sure DM is installed in your Kubernetes cluster and that the
Guestbook example is deployed by following the instructions in the top level
[README.md](../../README.md).

## Understanding the Guestbook example

Let's take a closer look at the configuration used by the Guestbook example.

### Replicated services

The typical design pattern for microservices in Kubernetes is to create a
replication controller and a service with the same selector, so that the service
exposes ports from the pods managed by the replication controller.

We have created a parameterized template for this kind of replicated service 
called [Replicated Service](../../templates/replicatedservice/v1), and we use it
three times in the Guestbook example.

The template is defined by a
[Python script](../../templates/replicatedservice/v1/replicatedservice.py). It 
also has a [schema](../../templates/replicatedservice/v1/replicatedservice.py.schema).
Schemas are optional. If provided, they are used to validate template invocations
that appear in configurations.

For more information about templates and schemas, see the
[design document](../../docs/design/design.md#templates).

### The Guestbook application
The Guestbook application consists of 2 microservices: a front end and a Redis 
cluster.

#### The front end

The front end is a replicated service with 3 replicas:

```
- name: frontend
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v1/replicatedservice.py
  properties:
    service_port: 80
    container_port: 80
    external_service: true
    replicas: 3
    image: gcr.io/google_containers/example-guestbook-php-redis:v3
```

(Note that we use the URL for a specific version of the template replicatedservice.py, 
not just the template name.)

#### The Redis cluster

The Redis cluster consists of two replicated services: a master with a single replica
and the slaves with 2 replicas. It's defined by [this template](../../templates/redis/v1/redis.jinja), 
which is a [Jinja](http://jinja.pocoo.org/) file with a [schema](../../templates/redis/v1/redis.jinja.schema).

```
{% set REDIS_PORT = 6379 %}
{% set WORKERS = properties['workers'] or 2 %}

resources:
- name: redis-master
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v1/replicatedservice.py
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
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v1/replicatedservice.py
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

You can see both the both primitive types and the templates you've deployed to the
cluster using the `deployed-types` command:

```
dm deployed-types 

["Service","ReplicationController","redis.jinja","https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v1/replicatedservice.py"]
```

This output shows 2 primitive types (Service and ReplicationController), and 2
templates (redis.jinja and one imported from github named replicatedservice.py).

You can also see where a specific type is being used with the `deployed-instances` command:

```
dm deployed-instances Service
[{"name":"frontend-service","type":"Service","deployment":"guestbook4","manifest":"manifest-1446682551242763329","path":"$.resources[0].resources[0]"},{"name":"redis-master","type":"Service","deployment":"guestbook4","manifest":"manifest-1446682551242763329","path":"$.resources[1].resources[0].resources[0]"},{"name":"redis-slave","type":"Service","deployment":"guestbook4","manifest":"manifest-1446682551242763329","path":"$.resources[1].resources[1].resources[0]"}]
```

This output describes the deployment and manifest, as well as the JSON paths to
the instances of the type within the layout.

For more information about deployments, manifests and layouts, see the
[design document](../../docs/design/design.md#api-model).


