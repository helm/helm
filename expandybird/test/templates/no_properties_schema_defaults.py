"""Basic firewall template."""


def GenerateConfig(evaluation_context):
  return """
resources:
- type: compute.v1.firewall
  name: %(master)s-firewall
  properties:
    sourceRanges: [ "0.0.0.0/0" ]
""" % {"master": evaluation_context.properties["firewallname"]}
