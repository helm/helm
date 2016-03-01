"""Constructs a VM."""


def GenerateConfig(_):
  """Generates config of a VM."""
  return """
resources:
- name: myBackend
  type: compute.v1.instance
  properties:
    machineSize: big
"""
