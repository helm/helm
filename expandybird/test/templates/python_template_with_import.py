# Copyright 2014 Google Inc. All Rights Reserved.

"""Constructs a VM."""
import json

import helpers.common
import helpers.extra.common2


def GenerateConfig(_):
  """Generates config of a VM."""
  return """
resources:
- name: %s
  type: compute.v1.instance
  properties:
    machineSize: %s
""" % (helpers.common.GenerateMachineName(
    json.dumps('myFrontend').strip('"'), 'prod'),
       helpers.extra.common2.GenerateMachineSize())
