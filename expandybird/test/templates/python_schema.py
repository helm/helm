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
- type: compute.v1.firewall
  name: %(master)s-firewall
  properties:
    network: https://www.googleapis.com/compute/v1/projects/%(project)s/global/networks/default
    sourceRanges: [ "0.0.0.0/0" ]
    allowed:
    - IPProtocol: tcp
      ports: [ "4505", "4506" ]
- type: compute.v1.instance
  name: %(master)s
  properties:
    zone: %(zone)s
    machineType: https://www.googleapis.com/compute/v1/projects/%(project)s/zones/%(zone)s/machineTypes/f1-micro
    disks:
    - deviceName: boot
      type: PERSISTENT
      boot: true
      autoDelete: true
      initializeParams:
        sourceImage: https://www.googleapis.com/compute/v1/projects/debian-cloud/global/images/debian-7-wheezy-v20140619
    networkInterfaces:
    - network: https://www.googleapis.com/compute/v1/projects/%(project)s/global/networks/default
      accessConfigs:
      - name: External NAT
        type: ONE_TO_ONE_NAT
    metadata:
      items:
      - key: startup-script
        value: startup-script-value
""" % {"master": evaluation_context.properties["masterAddress"],
       "project": evaluation_context.env["project"],
       "zone": evaluation_context.properties["zone"]}
