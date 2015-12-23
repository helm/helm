# Copyright 2014 Google Inc. All Rights Reserved.

"""Constructs a VM."""

# Verify that both ways of hierarchical imports work.
from helpers import common
import helpers.extra.common2


def GenerateConfig(evaluation_context):
  """Generates config of a VM."""

  resource = {}
  resource['name'] = common.GenerateMachineName('myFrontend', 'prod')
  resource['type'] = 'compute.v1.instance'
  resource['properties'] = {
      'description': evaluation_context.imports[
          evaluation_context.properties['description-file']],
      'machineSize': helpers.extra.common2.GenerateMachineSize()
  }

  return {'resources': [resource]}
