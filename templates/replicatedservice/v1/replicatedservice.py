######################################################################
# Copyright 2015 The Kubernetes Authors All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
######################################################################

"""Defines a ReplicatedService type by creating both a Service and an RC.

This module creates a typical abstraction for running a service in a
Kubernetes cluster, namely a replication controller and a service packaged
together into a single unit.
"""

import yaml

SERVICE_TYPE_COLLECTION = 'Service'
RC_TYPE_COLLECTION = 'ReplicationController'


def GenerateConfig(context):
  """Generates a Replication Controller and a matching Service.

  Args:
    context: Template context. See schema for context properties.

  Returns:
    A Container Manifest as a YAML string.
  """
  # YAML config that we're going to create for both RC & Service
  config = {'resources': []}

  name = context.env['name']
  container_name = context.properties.get('container_name', name)
  namespace = context.properties.get('namespace', 'default')

  # Define things that the Service cares about
  service_name = context.properties.get('service_name', name + '-service')
  service_type = SERVICE_TYPE_COLLECTION

  # Define things that the Replication Controller (rc) cares about
  rc_name = name + '-rc'
  rc_type = RC_TYPE_COLLECTION

  service = {
      'name': service_name,
      'type': service_type,
      'properties': {
          'apiVersion': 'v1',
          'kind': 'Service',
          'metadata': {
              'name': service_name,
              'namespace': namespace,
              'labels': GenerateLabels(context, service_name),
          },
          'spec': {
              'ports': [GenerateServicePorts(context, container_name)],
              'selector': GenerateLabels(context, name)
          }
      }
  }
  set_up_external_lb = context.properties.get('external_service', None)
  if set_up_external_lb:
    service['properties']['spec']['type'] = 'LoadBalancer'
  config['resources'].append(service)

  rc = {
      'name': rc_name,
      'type': rc_type,
      'properties': {
          'apiVersion': 'v1',
          'kind': 'ReplicationController',
          'metadata': {
              'name': rc_name,
              'namespace': namespace,
              'labels': GenerateLabels(context, rc_name),
          },
          'spec': {
              'replicas': context.properties['replicas'],
              'selector': GenerateLabels(context, name),
              'template': {
                  'metadata': {
                      'labels': GenerateLabels(context, name),
                  },
                  'spec': {
                      'containers': [
                          {
                              'env': GenerateEnv(context),
                              'name': container_name,
                              'image': context.properties['image'],
                              'ports': [
                                  {
                                      'name': container_name,
                                      'containerPort': context.properties['container_port'],
                                  }
                              ]
                          }
                      ]
                  }
              }
          }
      }
  }

  config['resources'].append(rc)
  return yaml.dump(config)


def GenerateLabels(context, name):
  """Generates labels either from the context.properties['labels'] or 
     generates a default label 'app':name

  We make a deep copy of the context.properties['labels'] section to avoid
  linking in the yaml document, which I believe reduces readability of the
  expanded template. If no labels are given, generate a default 'app':name.

  Args:
    context: Template context, which can contain the following properties:
             labels - Labels to generate

  Returns:
    A dict containing labels in a name:value format
  """
  tmp_labels = context.properties.get('labels', None)
  ret_labels = {'app': name}
  if isinstance(tmp_labels, dict):
    for key, value in tmp_labels.iteritems():
      ret_labels[key] = value
  return ret_labels


def GenerateServicePorts(context, name):
  """Generates a ports section for a service.

  Args:
    context: Template context, which can contain the following properties:
             service_port - Port to use for the service
             target_port - Target port for the service
             protocol - Protocol to use.

  Returns:
    A dict containing a port definition
  """
  service_port = context.properties.get('service_port', None)
  target_port = context.properties.get('target_port', None)
  protocol = context.properties.get('protocol')

  ports = {}
  if name:
    ports['name'] = name
  if service_port:
    ports['port'] = service_port
  if target_port:
    ports['targetPort'] = target_port
  if protocol:
    ports['protocol'] = protocol

  return ports

def GenerateEnv(context):
  """Generates environmental variables for a pod.

  Args:
    context: Template context, which can contain the following properties:
             env - Environment variables to set.

  Returns:
    A list containing env variables in dict format {name: 'name', value: 'value'}
  """
  env = []
  tmp_env = context.properties.get('env', [])
  for entry in tmp_env:
    if isinstance(entry, dict):
      env.append({'name': entry.get('name'), 'value': entry.get('value')})
  return env
