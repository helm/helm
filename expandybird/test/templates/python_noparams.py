# Copyright 2014 Google Inc. All Rights Reserved.

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
