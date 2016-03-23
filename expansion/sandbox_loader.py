# Copyright 2015 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#           http://www.apache.org/licenses/LICENSE-2.0
#
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


_IMPORTS = {}


class AllowedImportsLoader(object):

    def load_module(self, name, etc=None):  # pylint: disable=unused-argument
        """Implements loader.load_module()
        for loading user provided imports."""

        module = imp.new_module(name)
        content = _IMPORTS[name]

        if content is None:
            module.__path__ = [name.replace('.', '/')]
        else:
            # Run the module code.
            exec content in module.__dict__   # pylint: disable=exec-used

        # Register the module so Python code will find it.
        sys.modules[name] = module
        return module


class AllowedImportsHandler(object):

    def find_module(self, name, path=None):  # pylint: disable=unused-argument
        if name in _IMPORTS:
            return AllowedImportsLoader()
        else:
            return None  # Delegate to system handlers.


class FileAccessRedirector(object):

    @staticmethod
    def redirect(imports):
        """Restricts imports and builtin 'open' to the set of user provided imports.

        Imports already available in sys.modules will continue to be available.

        Args:
            imports: map from string to dict, the map of files from names.
        """
        if imports is not None:
            # Build map of fully qualified module names to either the content
            # of that module (if it is a file within a package) or just None if
            # the module is a package (i.e. a directory).
            for name, entry in imports.iteritems():
                path = entry['path']
                content = entry['content']
                prefix, ext = os.path.splitext(os.path.normpath(path))
                if ext not in {'.py', '.pyc'}:
                    continue
                if '.' in prefix:
                    # Python modules cannot contain '.', ignore these files.
                    continue
                parts = prefix.split(sep)
                dirs = ('.'.join(parts[0:i]) for i in xrange(0, len(parts)))
                for d in dirs:
                    if d not in _IMPORTS:
                        _IMPORTS[d] = None
                _IMPORTS['.'.join(parts)] = content

            # Prepend our module handler before standard ones.
            sys.meta_path = [AllowedImportsHandler()] + sys.meta_path
