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

"""App allowing expansion from file names instead of cmdline arguments."""
import os.path
import sys
from expansion import Expand


def main():
  if len(sys.argv) < 2:
    print >>sys.stderr, 'No template specified.'
    sys.exit(1)
  template = ''
  imports = {}
  try:
    with open(sys.argv[1]) as f:
      template = f.read()
    for imp in sys.argv[2:]:
      import_contents = ''
      with open(imp) as f:
        import_contents = f.read()
        import_name = os.path.basename(imp)
        imports[import_name] = import_contents
  except IOError as e:
    print 'IOException: ', str(e)
    sys.exit(1)

  env = {}
  env['deployment'] = os.environ['DEPLOYMENT_NAME']
  env['project'] = os.environ['PROJECT']
  validate_schema = 'VALIDATE_SCHEMA' in os.environ

  print  Expand(template, imports, env=env, validate_schema=validate_schema)


if __name__ == '__main__':
  main()
