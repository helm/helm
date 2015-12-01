# Copyright 2014 Google Inc. All Rights Reserved.

#% description: Creates a VM running a Salt master daemon in a Docker container.
#% parameters:
#% - name: masterAddress
#%   type: string
#%   description: Name of the Salt master VM.
#%   required: true
#% - name: project
#%   type: string
#%   description: Name of the Cloud project.
#%   required: true
#% - name: zone
#%   type: string
#%   description: Zone to create the resources in.
#%   required: true

"""Generates config for a VM running a SaltStack master.

Just for fun this template is in Python, while the others in this
directory are in Jinja2.
"""


def GenerateConfig(evaluation_context):
  return """
resources:
- name: python_and_jinja_template_jinja_name
  type: python_and_jinja_template.jinja
  properties:
    zone: %(zone)s
    project: %(project)s
    deployment: %(master)s

""" % {"master": evaluation_context.properties["masterAddress"],
       "project": evaluation_context.properties["project"],
       "zone": evaluation_context.properties["zone"]}
