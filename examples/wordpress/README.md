# Wordpress Example
<<<<<<< HEAD
<<<<<<< HEAD
Welcome to the Wordpress example. It shows you how to deploy a Wordpress application using Deployment Manager.

## Prerequisites
=======

Welcome to the Wordpress example. It shows you how to deploy a Wordpress application using Deployment Manager.

## Prerequisites


>>>>>>> 320a0e9... Adds Wordpress example
=======
Welcome to the Wordpress example. It shows you how to deploy a Wordpress application using Deployment Manager.

## Prerequisites
>>>>>>> eed0dda... Updates Wordpress README for Kubernetes 1.1 release
### Deployment Manager
First, make sure DM is installed in your Kubernetes cluster by following the instructions in the top level
[README.md](../../README.md).

### Google Cloud Resources
<<<<<<< HEAD
<<<<<<< HEAD
The Wordpress application will make use of several persistent disks, which we will host on Google Cloud. To create these disks we will create a deployment using Google Cloud Deployment Manager:
=======
The Wordpress application will make use of several persistent disks, which we will host on Google Cloud. To create these disks we will create a deployment using Google Cloud Deployment Manager: 
>>>>>>> 320a0e9... Adds Wordpress example
=======
The Wordpress application will make use of several persistent disks, which we will host on Google Cloud. To create these disks we will create a deployment using Google Cloud Deployment Manager:
>>>>>>> f7940f1... Updates Wordpress example
```gcloud deployment-manager deployments create wordpress-resources --config wordpress-resources.yaml```

where `wordpress-resources.yaml` looks as follows:

```
resources:
- name: nfs-disk
  type: compute.v1.disk
  properties:
    zone: us-central1-b
    sizeGb: 200
- name: mysql-disk
  type: compute.v1.disk
  properties:
    zone: us-central1-b
    sizeGb: 200
```

### Privileged containers
<<<<<<< HEAD
<<<<<<< HEAD
To use NFS we need to be able to launch privileged containers. Since the release of Kubernetes 1.1 privileged container support is enabled by default. If your Kubernetes cluster doesn't support privileged containers you need to manually change this by setting the flag at `kubernetes/saltbase/pillar/privilege.sls` to true.

### NFS Library
Mounting NFS volumes requires NFS libraries. Since the release of Kubernetes 1.1 the NFS libraries are installed by default. If they are not installed on your Kubernetes cluster you need to install them manually.

## Understanding the Wordpress example template
=======
To use NFS we need to be able to launch privileged containers. If your Kubernetes cluster doesn't support this you can either manually change this in every Kubernetes minion or you could set the flag at `kubernetes/saltbase/pillar/privilege.sls` to true and (re)launch your Kubernetes cluster. Once Kubernetes 1.1 releases this should be enabled by default.
=======
To use NFS we need to be able to launch privileged containers. Since the release of Kubernetes 1.1 privileged container support is enabled by default. If your Kubernetes cluster doesn't support privileged containers you need to manually change this by setting the flag at `kubernetes/saltbase/pillar/privilege.sls` to true.
>>>>>>> eed0dda... Updates Wordpress README for Kubernetes 1.1 release

### NFS Library
Mounting NFS volumes requires NFS libraries. Since the release of Kubernetes 1.1 the NFS libraries are installed by default. If they are not installed on your Kubernetes cluster you need to install them manually.

## Understanding the Wordpress example template
<<<<<<< HEAD

>>>>>>> 320a0e9... Adds Wordpress example
=======
>>>>>>> eed0dda... Updates Wordpress README for Kubernetes 1.1 release
Let's take a closer look at the template used by the Wordpress example. The Wordpress application consists of 4 microservices: an nginx service, a wordpress-php service, a MySQL service, and an NFS service. The architecture looks as follows:

![Architecture](architecture.png)

### Variables
The template contains the following variables:

```
<<<<<<< HEAD
<<<<<<< HEAD
{% set PROPERTIES = properties or {} %}
{% set PROJECT = PROPERTIES['project'] or 'dm-k8s-testing' %}
{% set NFS_SERVER = PROPERTIES['nfs-server'] or {} %}
=======
{% set PROJECT = properties['project'] or 'dm-k8s-testing' %}
{% set NFS_SERVER = properties['nfs-server'] or {} %}
>>>>>>> 320a0e9... Adds Wordpress example
=======
{% set PROPERTIES = properties or {} %}
{% set PROJECT = PROPERTIES['project'] or 'dm-k8s-testing' %}
{% set NFS_SERVER = PROPERTIES['nfs-server'] or {} %}
>>>>>>> f7940f1... Updates Wordpress example
{% set NFS_SERVER_IP = NFS_SERVER['ip'] or '10.0.253.247' %}
{% set NFS_SERVER_PORT = NFS_SERVER['port'] or 2049 %}
{% set NFS_SERVER_DISK = NFS_SERVER['disk'] or 'nfs-disk' %}
{% set NFS_SERVER_DISK_FSTYPE = NFS_SERVER['fstype'] or 'ext4' %}
<<<<<<< HEAD
<<<<<<< HEAD
{% set NGINX = PROPERTIES['nginx'] or {} %}
{% set NGINX_PORT = 80 %}
{% set NGINX_REPLICAS = NGINX['replicas'] or 2 %}
{% set WORDPRESS_PHP = PROPERTIES['wordpress-php'] or {} %}
{% set WORDPRESS_PHP_REPLICAS = WORDPRESS_PHP['replicas'] or 2 %}
{% set WORDPRESS_PHP_PORT = WORDPRESS_PHP['port'] or 9000 %}
{% set MYSQL = PROPERTIES['mysql'] or {} %}                                                                                                                                                                                                                                 {% set MYSQL_PORT = MYSQL['port'] or 3306 %}                                                                                                                                                                                                                                {% set MYSQL_PASSWORD = MYSQL['password'] or 'mysql-password' %}                                                                                                                                                                                                            {% set MYSQL_DISK = MYSQL['disk'] or 'mysql-disk' %}                                                                                                                                                                                                                        {% set MYSQL_DISK_FSTYPE = MYSQL['fstype'] or 'ext4' %}
=======
{% set NGINX = properties['nginx'] or {} %}
=======
{% set NGINX = PROPERTIES['nginx'] or {} %}
>>>>>>> f7940f1... Updates Wordpress example
{% set NGINX_PORT = 80 %}
{% set NGINX_REPLICAS = NGINX['replicas'] or 2 %}
{% set WORDPRESS_PHP = PROPERTIES['wordpress-php'] or {} %}
{% set WORDPRESS_PHP_REPLICAS = WORDPRESS_PHP['replicas'] or 2 %}
{% set WORDPRESS_PHP_PORT = WORDPRESS_PHP['port'] or 9000 %}
<<<<<<< HEAD
{% set MYSQL = properties['mysql'] or {} %}
{% set MYSQL_PORT = MYSQL['port'] or 3306 %}
{% set MYSQL_PASSWORD = MYSQL['password'] or 'mysql-password' %}
{% set MYSQL_DISK = MYSQL['disk'] or 'mysql-disk' %}
{% set MYSQL_DISK_FSTYPE = MYSQL['fstype'] or 'ext4' %}
>>>>>>> 320a0e9... Adds Wordpress example
=======
{% set MYSQL = PROPERTIES['mysql'] or {} %}                                                                                                                                                                                                                                 {% set MYSQL_PORT = MYSQL['port'] or 3306 %}                                                                                                                                                                                                                                {% set MYSQL_PASSWORD = MYSQL['password'] or 'mysql-password' %}                                                                                                                                                                                                            {% set MYSQL_DISK = MYSQL['disk'] or 'mysql-disk' %}                                                                                                                                                                                                                        {% set MYSQL_DISK_FSTYPE = MYSQL['fstype'] or 'ext4' %}
>>>>>>> f7940f1... Updates Wordpress example
```

### Nginx service
The nginx service is a replicated service with 2 replicas:

```
- name: nginx
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
=======
  type: https://raw.githubusercontent.com/leendersr/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> 320a0e9... Adds Wordpress example
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> ebf3a33... Change hardcoded type urls from leendersr/deployment-manager to kubernetes/deployment-manager
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> 77312ee... Updates README to link to master/templates
=======
  type: https://raw.githubusercontent.com/leendersr/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> af6b5a7... Updates README and adds clain-name property to the NFS type
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> be9b2a3... Changes absolute url from leendersr/deployment-manager back to kubernetes/deployment-manager
  properties:
    service_port: {{ NGINX_PORT }}
    container_port: {{ NGINX_PORT }}
    replicas: {{ NGINX_REPLICAS }}
    external_service: true
    image: gcr.io/{{ PROJECT }}/nginx:latest
    volumes:
      - mount_path: /var/www/html
        persistentVolumeClaim:
          claimName: nfs
```

The nginx image builds upon the standard nginx image and simply copies a custom configuration file.

### Wordpress-php service
The wordpress-php service is a replicated service with 2 replicas:

```
- name: wordpress-php
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
=======
  type: https://raw.githubusercontent.com/leendersr/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> 320a0e9... Adds Wordpress example
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> ebf3a33... Change hardcoded type urls from leendersr/deployment-manager to kubernetes/deployment-manager
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> 77312ee... Updates README to link to master/templates
=======
  type: https://raw.githubusercontent.com/leendersr/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> af6b5a7... Updates README and adds clain-name property to the NFS type
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> be9b2a3... Changes absolute url from leendersr/deployment-manager back to kubernetes/deployment-manager
  properties:
    service_name: wordpress-php
    service_port: {{ WORDPRESS_PHP_PORT }}
    container_port: {{ WORDPRESS_PHP_PORT }}
    replicas: 2
    image: wordpress:fpm
    env:
      - name: WORDPRESS_DB_PASSWORD
        value: {{ MYSQL_PASSWORD }}
      - name: WORDPRESS_DB_HOST
        value: mysql-service
    volumes:
      - mount_path: /var/www/html
        persistentVolumeClaim:
          claimName: nfs
```

### MySQL service
The MySQL service is a replicated service with a single replica:

```
- name: mysql
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
=======
  type: https://raw.githubusercontent.com/leendersr/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> 320a0e9... Adds Wordpress example
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> ebf3a33... Change hardcoded type urls from leendersr/deployment-manager to kubernetes/deployment-manager
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> 77312ee... Updates README to link to master/templates
  properties:
    service_port: {{ MYSQL_PORT }}
    container_port: {{ MYSQL_PORT }}
    replicas: 1
    image: mysql:5.6
    env:
      - name: MYSQL_ROOT_PASSWORD
        value: {{ MYSQL_PASSWORD }}
    volumes:
      - mount_path: /var/lib/mysql
        gcePersistentDisk:
          pdName: {{ MYSQL_DISK }}
          fsType: {{ MYSQL_DISK_FSTYPE }}
<<<<<<< HEAD
<<<<<<< HEAD
```
=======
```         
>>>>>>> 320a0e9... Adds Wordpress example
=======
```
>>>>>>> f7940f1... Updates Wordpress example

### NFS service
The NFS service is a replicated service with a single replica that is available as a type:

```
<<<<<<< HEAD
- name: nfs-server
<<<<<<< HEAD
<<<<<<< HEAD
<<<<<<< HEAD
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
=======
  type: https://raw.githubusercontent.com/leendersr/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> 320a0e9... Adds Wordpress example
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/types/replicatedservice/v2/replicatedservice.py
>>>>>>> ebf3a33... Change hardcoded type urls from leendersr/deployment-manager to kubernetes/deployment-manager
=======
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/replicatedservice/v2/replicatedservice.py
>>>>>>> 77312ee... Updates README to link to master/templates
  properties:
    service_port: {{ NFS_SERVER_PORT }}
    container_port: {{ NFS_SERVER_PORT }}
    replicas: 1 # Has to be 1 because of the persistent disk
    image: jsafrane/nfs-data
    privileged: true
    cluster_ip: {{ NFS_SERVER_IP }}
    volumes:
      - mount_path: /mnt/data
        gcePersistentDisk:
          pdName: {{ NFS_SERVER_DISK }}
          fsType: {{ NFS_SERVER_DISK_FSTYPE }}
<<<<<<< HEAD
<<<<<<< HEAD
=======
- name: nfs
  type: https://raw.githubusercontent.com/kubernetes/deployment-manager/master/templates/nfs/v1/nfs.jinja
  properties:
    ip: {{ NFS_SERVER_IP }}
    port: {{ NFS_SERVER_PORT }}
    disk: {{ NFS_SERVER_DISK }}
    fstype: {{NFS_SERVER_DISK_FSTYPE }}
>>>>>>> af6b5a7... Updates README and adds clain-name property to the NFS type
```
=======
```          
>>>>>>> 320a0e9... Adds Wordpress example
=======
```
>>>>>>> f7940f1... Updates Wordpress example

## Deploying Wordpress
We can now deploy Wordpress using:

```
<<<<<<< HEAD
<<<<<<< HEAD
dm deploy examples/wordpress/wordpress.yaml
=======
dm deploy examples/wordpress/wordpress.jinja
>>>>>>> 320a0e9... Adds Wordpress example
=======
dm deploy examples/wordpress/wordpress.yaml
>>>>>>> ef7e062... Fix typo, we want to deploy wordpress.yaml not .jinja
```

where `wordpress.yaml` looks as follows:

```
imports:
- path: wordpress.jinja

resources:
- name: wordpress
  type: wordpress.jinja
  properties:
    project: <YOUR PROJECT>
```
