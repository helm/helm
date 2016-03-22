#!/usr/bin/env python
#
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

"""Template expansion utilities."""

import os.path
import sys
import traceback

import jinja2
import yaml

from sandbox_loader import FileAccessRedirector

import schema_validation


def Expand(config, imports=None, env=None, validate_schema=False):
    """Expand the configuration with imports.

    Args:
        config: string, the raw config to be expanded.
        imports: map from import file name, e.g. "helpers/constants.py" to
                its contents.
        env: map from string to string, the map of environment variable names
                to their values
        validate_schema: True to run schema validation; False otherwise
    Returns:
        YAML containing the expanded configuration and its layout,
        in the following format:

            config:
                ...
            layout:
                ...

    Raises:
        ExpansionError: if there is any error occurred during expansion
    """
    try:
        return _Expand(config, imports=imports, env=env,
                       validate_schema=validate_schema)
    except Exception as e:
        # print traceback.format_exc()
        raise ExpansionError('config', str(e))


def _Expand(config, imports=None, env=None, validate_schema=False):
    """Expand the configuration with imports."""

    FileAccessRedirector.redirect(imports)

    yaml_config = None
    try:
        yaml_config = yaml.safe_load(config)
    except yaml.scanner.ScannerError as e:
        # Here we know that YAML parser could not parse the template
        # we've given it. YAML raises a ScannerError that specifies which file
        # had the problem, as well as line and column, but since we're giving
        # it the template from string, error message contains <string>, which
        # is not very helpful on the user end, so replace it with word
        # "template" and make it obvious that YAML contains a syntactic error.
        msg = str(e).replace('"<string>"', 'template')
        raise Exception('Error parsing YAML: %s' % msg)

    # Handle empty file case
    if not yaml_config:
        return ''

    # If the configuration does not have ':' in it, the yaml_config will be a
    # string. If this is the case just return the str. The code below it
    # assumes yaml_config is a map for common cases.
    if type(yaml_config) is str:
        return yaml_config

    if 'resources' not in yaml_config or yaml_config['resources'] is None:
        yaml_config['resources'] = []

    config = {'resources': []}
    layout = {'resources': []}

    _ValidateUniqueNames(yaml_config['resources'])

    # Iterate over all the resources to process.
    for resource in yaml_config['resources']:
        processed_resource = _ProcessResource(resource, imports, env,
                                              validate_schema)

        config['resources'].extend(processed_resource['config']['resources'])
        layout['resources'].append(processed_resource['layout'])

    result = {'config': config, 'layout': layout}
    return yaml.safe_dump(result, default_flow_style=False)


def _ProcessResource(resource, imports, env, validate_schema=False):
    """Processes a resource and expands if template.

    Args:
        resource: the resource to be processed, as a map.
        imports: map from string to string, the map of imported files names
                and contents
        env: map from string to string, the map of environment variable names
                to their values
        validate_schema: True to run schema validation; False otherwise
    Returns:
        A map containing the layout and configuration of the expanded
        resource and any sub-resources, in the format:

        {'config': ..., 'layout': ...}
    Raises:
        ExpansionError: if there is any error occurred during expansion
    """
    # A resource has to have to a name.
    if 'name' not in resource:
        raise ExpansionError(resource, 'Resource does not have a name.')

    # A resource has to have a type.
    if 'type' not in resource:
        raise ExpansionError(resource, 'Resource does not have type defined.')

    config = {'resources': []}
    # Initialize layout with basic resource information.
    layout = {'name': resource['name'],
              'type': resource['type']}

    if resource['type'] in imports:
        # A template resource, which contains sub-resources.
        expanded_template = ExpandTemplate(resource, imports,
                                           env, validate_schema)

        if expanded_template['resources']:
            _ValidateUniqueNames(expanded_template['resources'],
                                 resource['type'])

            # Process all sub-resources of this template.
            for resource_to_process in expanded_template['resources']:
                processed_resource = _ProcessResource(resource_to_process,
                                                      imports, env,
                                                      validate_schema)

                # Append all sub-resources to the config resources,
                # and the resulting layout of sub-resources.
                config['resources'].extend(processed_resource['config']
                                                             ['resources'])

                # Lazy-initialize resources key here because it is not set for
                # non-template layouts.
                if 'resources' not in layout:
                    layout['resources'] = []
                layout['resources'].append(processed_resource['layout'])

                if 'properties' in resource:
                    layout['properties'] = resource['properties']
    else:
        # A normal resource has only itself for config.
        config['resources'] = [resource]

    return {'config': config,
            'layout': layout}


def _ValidateUniqueNames(template_resources, template_name='config'):
    """Make sure that every resource name in the given template is unique."""
    names = set()
    # Validate that every resource name is unique
    for resource in template_resources:
        if 'name' in resource:
            if resource['name'] in names:
                raise ExpansionError(
                        resource,
                        'Resource name \'%s\' is not unique in %s.'
                        % (resource['name'], template_name))
            names.add(resource['name'])
        # If this resource doesn't have a name, we will report that error later


def ExpandTemplate(resource, imports, env, validate_schema=False):
    """Expands a template, calling expansion mechanism based on type.

    Args:
        resource: resource object, the resource that contains parameters to the
                jinja file
        imports: map from string to string, the map of imported files names
                and contents
        env: map from string to string, the map of environment variable names
                to their values
        validate_schema: True to run schema validation; False otherwise
    Returns:
        The final expanded template

    Raises:
        ExpansionError: if there is any error occurred during expansion
    """
    source_file = resource['type']
    path = resource['type']

    # Look for Template in imports.
    if source_file not in imports:
        raise ExpansionError(
                source_file,
                'Unable to find source file %s in imports.' % (source_file))

    # source_file could be a short version of the template
    # say github short name) so we need to potentially map this into
    # the fully resolvable name.
    if 'path' in imports[source_file] and imports[source_file]['path']:
            path = imports[source_file]['path']

    resource['imports'] = imports

    # Populate the additional environment variables.
    if env is None:
        env = {}
    env['name'] = resource['name']
    env['type'] = resource['type']
    resource['env'] = env

    schema = source_file + '.schema'
    if validate_schema and schema in imports:
        properties = resource['properties'] if 'properties' in resource else {}
        try:
            resource['properties'] = schema_validation.Validate(
                    properties, schema, source_file, imports)
        except schema_validation.ValidationErrors as e:
            raise ExpansionError(resource['name'], e.message)

    if path.endswith('jinja') or path.endswith('yaml'):
        expanded_template = ExpandJinja(
            source_file, imports[source_file]['content'], resource, imports)
    elif path.endswith('py'):
        # This is a Python template.
        expanded_template = ExpandPython(
                imports[source_file]['content'], source_file, resource)
    else:
        # The source file is not a jinja file or a python file.
        # This in fact should never happen due to the IsTemplate check above.
        raise ExpansionError(
                resource['source'],
                'Unsupported source file: %s.' % (source_file))

    parsed_template = yaml.safe_load(expanded_template)

    if parsed_template is None or 'resources' not in parsed_template:
        raise ExpansionError(resource['type'],
                             'Template did not return a \'resources:\' field.')

    return parsed_template


def ExpandJinja(file_name, source_template, resource, imports):
    """Render the jinja template using jinja libraries.

    Args:
        file_name:
            string, the file name.
        source_template:
            string, the content of jinja file to be render
        resource:
            resource object, the resource that contains parameters to the
            jinja file
        imports:
            map from string to map {name, path}, the map of imported
            files names fully resolved path and contents
    Returns:
        The final expanded template
    Raises:
        ExpansionError in case we fail to expand the Jinja2 template.
    """

    try:
        jinja_imports = { i['path']: i['content'] for _, i in imports.iteritems() }
        env = jinja2.Environment(loader=jinja2.DictLoader(jinja_imports))

        template = env.from_string(source_template)

        if ('properties' in resource or 'env' in resource or
                'imports' in resource):
            return template.render(resource)
        else:
            return template.render()
    except Exception:
        st = 'Exception in %s\n%s' % (file_name, traceback.format_exc())
        raise ExpansionError(file_name, st)


def ExpandPython(python_source, file_name, params):
    """Run python script to get the expanded template.

    Args:
        python_source: string, the python source file to run
        file_name: string, the name of the python source file
        params: object that contains 'imports' and 'params', the parameters to
                the python script
    Returns:
        The final expanded template.
    """

    try:
        # Compile the python code to be run.
        constructor = {}
        compiled_code = compile(python_source, '<string>', 'exec')
        exec compiled_code in constructor  # pylint: disable=exec-used

        # Construct the parameters to the python script.
        evaluation_context = PythonEvaluationContext(params)

        return constructor['GenerateConfig'](evaluation_context)
    except Exception:
        st = 'Exception in %s\n%s' % (file_name, traceback.format_exc())
        raise ExpansionError(file_name, st)


class PythonEvaluationContext(object):
    """The python evaluation context.

    Attributes:
            params -- the parameters to be used in the expansion
    """

    def __init__(self, params):
        if 'properties' in params:
            self.properties = params['properties']
        else:
            self.properties = None

        if 'imports' in params:
            self.imports = params['imports']
        else:
            self.imports = None

        if 'env' in params:
            self.env = params['env']
        else:
            self.env = None


class ExpansionError(Exception):
    """Exception raised for errors during expansion process.

    Attributes:
        resource: the resource processed that results in the error
        message: the detailed message of the error
    """

    def __init__(self, resource, message):
        self.resource = resource
        self.message = message + ' Resource: ' + str(resource)
        super(ExpansionError, self).__init__(self.message)


def main():
    if len(sys.argv) < 2:
        print >> sys.stderr, 'No input specified.'
        sys.exit(1)
    template = sys.argv[1]
    idx = 2
    imports = {}
    while idx < len(sys.argv):
        if idx + 1 == len(sys.argv):
            print >>sys.stderr, 'Invalid import definition at argv pos %d' \
                                 % idx
            sys.exit(1)
        name = sys.argv[idx]
        path = sys.argv[idx + 1]
        value = sys.argv[idx + 2]
        imports[name] = {'content': value, 'path': path}
        idx += 3

    env = {}
    # env['deployment'] = os.environ['DEPLOYMENT_NAME']
    # env['project'] = os.environ['PROJECT']

    validate_schema = 'VALIDATE_SCHEMA' in os.environ

    # Call the expansion logic to actually expand the template.
    print Expand(template, imports, env=env, validate_schema=validate_schema)

if __name__ == '__main__':
    main()
