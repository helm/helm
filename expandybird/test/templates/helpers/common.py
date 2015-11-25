# Copyright 2014 Google Inc. All Rights Reserved.

"""Dummy helper methods invoked in other constructors."""


def GenerateMachineName(prefix, suffix):
  """Generates name of a VM."""
  return prefix + "-" + suffix
