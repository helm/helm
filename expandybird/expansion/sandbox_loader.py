######################################################################
# Copyright 2015 The Kubernetes Authors All rights reserved.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
######################################################################

"""Loader for loading modules from a user provided dictionary of imports."""

import imp
from os import sep
import os.path
import sys


class AllowedImportsLoader(object):
  # Dictionary with modules loaded from user provided imports
  user_modules = {}

  @staticmethod
  def get_filename(name):
    return '%s.py' % name.replace('.', '/')

  def load_module(self, name, etc=None):  # pylint: disable=unused-argument
    """Implements loader.load_module() for loading user provided imports."""

    if name in AllowedImportsLoader.user_modules:
      return AllowedImportsLoader.user_modules[name]

    module = imp.new_module(name)

    try:
      data = FileAccessRedirector.allowed_imports[self.get_filename(name)]
    except Exception:  # pylint: disable=broad-except
      return None

    # Run the module code.
    exec data in module.__dict__  # pylint: disable=exec-used

    AllowedImportsLoader.user_modules[name] = module

    # We need to register the module in module registry, since new_module
    # doesn't do this, but we need it for hierarchical references.
    sys.modules[name] = module

    # If this module has children load them recursively.
    if name in FileAccessRedirector.parents:
      for child in FileAccessRedirector.parents[name]:
        full_name = name + '.' + child
        self.load_module(full_name)
        # If we have helpers/common.py package, then for it to be successfully
        # resolved helpers.common name must resolvable, hence, once we load
        # child package we attach it to parent module immeadiately.
        module.__dict__[child] = AllowedImportsLoader.user_modules[full_name]
    return module


class AllowedImportsHandler(object):

  def find_module(self, name, path=None):  # pylint: disable=unused-argument
    filename = AllowedImportsLoader.get_filename(name)

    if filename in FileAccessRedirector.allowed_imports:
      return AllowedImportsLoader()
    else:
      return None


def process_imports(imports):
  """Processes the imports by copying them and adding necessary parent packages.

    Copies the imports and then for all the hierarchical packages creates
    dummy entries for those parent packages, so that hierarchical imports
    can be resolved. In the process parent child relationship map is built.
    For example: helpers/extra/common.py will generate helpers, helpers.extra
    and helpers.extra.common packages along with related .py files.

  Args:
    imports: map of files to their relative paths.
  Returns:
    dictionary of imports to their contents and parent-child pacakge
    relationship map.
  """
  # First clone all the existing ones.
  ret = {}
  parents = {}
  for k in imports:
    ret[k] = imports[k]

  # Now build the hierarchical modules.
  for k in imports.keys():
    if imports[k]['path'].endswith('.jinja'):
      continue
    # Normalize paths and trim .py extension, if any.
    normalized = os.path.splitext(os.path.normpath(k))[0]
    # If this is actually a path and not an absolute name, split it and process
    # the hierarchical packages.
    if sep in normalized:
      parts = normalized.split(sep)
      # Create dummy file entries for package levels and also retain
      # parent-child relationships.
      for i in xrange(0, len(parts)-1):
        # Generate the partial package path.
        path = os.path.join(parts[0], *parts[1:i+1])
        # __init__.py file might have been provided and non-empty by the user.
        if path not in ret:
          # exec requires at least new line to be present to successfully
          # compile the file.
          ret[path + '.py'] = '\n'
        else:
          # To simplify our code, we'll store both versions in that case, since
          # loader code expects files with .py extension.
          ret[path + '.py'] = ret[path]
        # Generate fully qualified package name.
        fqpn = '.'.join(parts[0:i+1])
        if fqpn in parents:
          parents[fqpn].append(parts[i+1])
        else:
          parents[fqpn] = [parts[i+1]]
  return ret, parents


class FileAccessRedirector(object):
  # Dictionary with user provided imports.
  allowed_imports = {}
  # Dictionary that shows parent child relationships, key is the parent, value
  # is the list of child packages.
  parents = {}

  @staticmethod
  def redirect(imports):
    """Restricts imports and builtin 'open' to the set of user provided imports.

    Imports already available in sys.modules will continue to be available.

    Args:
      imports: map from string to string, the map of imported files names
          and contents.
    """
    if imports is not None:
      imps, parents = process_imports(imports)
      FileAccessRedirector.allowed_imports = imps
      FileAccessRedirector.parents = parents

      # Prepend our module handler before standard ones.
      sys.meta_path = [AllowedImportsHandler()] + sys.meta_path

