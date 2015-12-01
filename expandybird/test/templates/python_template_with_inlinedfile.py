# Copyright 2014 Google Inc. All Rights Reserved.

"""Constructs a VM."""

# Verify that both ways of hierarchical imports work.
from helpers import common
import helpers.extra.common2


def GenerateConfig(evaluation_context):
  """Generates config of a VM."""
  return """
resources:
- name: %s
  type: compute.v1.instance
  properties:
    description: %s
    machineSize: %s
""" % (common.GenerateMachineName("myFrontend", "prod"),
       evaluation_context.imports[
           evaluation_context.properties["description-file"]],
       helpers.extra.common2.GenerateMachineSize())
