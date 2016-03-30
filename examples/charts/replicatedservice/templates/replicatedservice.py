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
    context: Template context. See schema for context properties

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
  rc_name = context.properties.get('rc_name', name + '-rc')
  rc_type = RC_TYPE_COLLECTION

  service = {
      'name': service_name,
      'properties': {
          'apiVersion': 'v1',
          'kind': 'Service',
          'metadata': {
              'labels': GenerateLabels(context, service_name),
              'name': service_name,
              'namespace': namespace,
          },
          'spec': {
              'ports': [GenerateServicePorts(context, container_name)],
              'selector': GenerateLabels(context, name)
          }
      },
      'type': service_type,
  }
  set_up_external_lb = context.properties.get('external_service', None)
  if set_up_external_lb:
    service['properties']['spec']['type'] = 'LoadBalancer'
  cluster_ip = context.properties.get('cluster_ip', None)
  if cluster_ip:
    service['properties']['spec']['clusterIP'] = cluster_ip

  rc = {
      'name': rc_name,
      'type': rc_type,
      'properties': {
          'apiVersion': 'v1',
          'kind': 'ReplicationController',
          'metadata': {
              'labels': GenerateLabels(context, rc_name),
              'name': rc_name,
              'namespace': namespace,
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
                              'image': context.properties['image'],
                              'name': container_name,
                              'ports': [
                                  {
                                      'name': container_name,
                                      'containerPort': context.properties['container_port'],
                                  }
                              ],
                          }
                      ],
                  }
              }
          }
      }
  }

  # Set up volume mounts
  if context.properties.get('volumes', None):
    rc['properties']['spec']['template']['spec']['containers'][0]['volumeMounts'] = []
    rc['properties']['spec']['template']['spec']['volumes'] = []
    for volume in context.properties['volumes']:
      # mountPath should be unique
      volume_name = volume['mount_path'].replace('/', '-').lstrip('-') + '-storage'
      rc['properties']['spec']['template']['spec']['containers'][0]['volumeMounts'].append(
          {
              'name': volume_name,
              'mountPath': volume['mount_path']
          }
      )
      del volume['mount_path']
      volume['name'] = volume_name
      rc['properties']['spec']['template']['spec']['volumes'].append(volume)

  if context.properties.get('privileged', False):
    rc['properties']['spec']['template']['spec']['containers'][0]['securityContext'] = {
        'privileged': True
    }

  config['resources'].append(rc)
  config['resources'].append(service)
  return yaml.dump(config)


# Generates labels either from the context.properties['labels'] or generates
# a default label 'name':name
def GenerateLabels(context, name):
  """Generates labels from context.properties['labels'] or creates default.

  We make a deep copy of the context.properties['labels'] section to avoid
  linking in the yaml document, which I believe reduces readability of the
  expanded template. If no labels are given, generate a default 'name':name.

  Args:
    context: Template context, which can contain the following properties:
             labels - Labels to generate

  Returns:
    A dict containing labels in a name:value format
  """
  tmp_labels = context.properties.get('labels', None)
  ret_labels = {'name': name}
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
  container_port = context.properties['container_port']
  target_port = context.properties.get('target_port', container_port)
  service_port = context.properties.get('service_port', target_port)
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
