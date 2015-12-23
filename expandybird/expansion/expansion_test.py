# Copyright 2015 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Basic unit tests for template expansion library."""

import expansion
import os
import unittest
import yaml


def GetFilePath():
  """Find our source and data files."""
  return  os.path.dirname(os.path.abspath(__file__))


def ReadTestFile(filename):
  """Returns contents of a file from the test/ directory."""

  full_path = GetFilePath() + '/../test/templates/' + filename
  test_file = open(full_path, 'r')
  return test_file.read()


def ReadImportFile(filename):
  """Returns {'content' : value} of a file from the test/ directory."""

  full_path = GetFilePath() + '/../test/templates/' + filename
  test_file = open(full_path, 'r')
  return {'content': test_file.read(),
          'path': full_path}


def GetTestBasePath(filename):
  """Returns the base path of a file from the testdata/ directory."""

  full_path = GetFilePath() + '/../test/templates/' + filename
  return os.path.dirname(full_path)


class ExpansionTest(unittest.TestCase):
  """Tests basic functionality of the template expansion library."""

  EMPTY_RESPONSE = 'config:\n  resources: []\nlayout:\n  resources: []\n'

  def testEmptyExpansion(self):
    template = ''
    expanded_template = expansion.Expand(
        template)

    self.assertEqual('', expanded_template)

  def testNoResourcesList(self):
    template = 'imports: [ test.import ]'
    expanded_template = expansion.Expand(
        template)

    self.assertEqual(self.EMPTY_RESPONSE, expanded_template)

  def testResourcesListEmpty(self):
    template = 'resources:'
    expanded_template = expansion.Expand(
        template)

    self.assertEqual(self.EMPTY_RESPONSE, expanded_template)

  def testSimpleNoExpansionTemplate(self):
    template = ReadTestFile('simple.yaml')

    expanded_template = expansion.Expand(
        template)

    result_file = ReadTestFile('simple_result.yaml')
    self.assertEquals(result_file, expanded_template)

  def testJinjaExpansion(self):
    template = ReadTestFile('jinja_template.yaml')

    imports = {}
    imports['jinja_template.jinja'] = ReadImportFile('jinja_template.jinja')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('jinja_template_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testJinjaWithNoParamsExpansion(self):
    template = ReadTestFile('jinja_noparams.yaml')

    imports = {}
    imports['jinja_noparams.jinja'] = ReadImportFile('jinja_noparams.jinja')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('jinja_noparams_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonWithNoParamsExpansion(self):
    template = ReadTestFile('python_noparams.yaml')

    imports = {}
    imports['python_noparams.py'] = ReadImportFile('python_noparams.py')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('python_noparams_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonExpansion(self):
    template = ReadTestFile('python_template.yaml')

    imports = {}
    imports['python_template.py'] = ReadImportFile('python_template.py')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('python_template_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonAndJinjaExpansion(self):
    template = ReadTestFile('python_and_jinja_template.yaml')

    imports = {}
    imports['python_and_jinja_template.py'] = ReadImportFile(
        'python_and_jinja_template.py')

    imports['python_and_jinja_template.jinja'] = ReadImportFile(
        'python_and_jinja_template.jinja')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('python_and_jinja_template_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testNoImportErrors(self):
    # TODO(grahamawelch): Ask Ville what whether this test should expect an
    # expansion failure or not...
    template = 'resources: \n- type: something.jinja\n  name: something'
    # expansion.Expand(template, {})
    # Maybe it should fail, maybe it shouldn't...

  def testInvalidConfig(self):
    template = ReadTestFile('invalid_config.yaml')

    try:
      expansion.Expand(
          template)
      self.fail('Expansion should fail')
    except Exception as e:
      self.assertIn('Error parsing YAML', e.message)

  def testJinjaWithImport(self):
    template = ReadTestFile('jinja_template_with_import.yaml')

    imports = {}
    imports['jinja_template_with_import.jinja'] = ReadImportFile(
        'jinja_template_with_import.jinja')
    imports['helpers/common.jinja'] = ReadImportFile(
        'helpers/common.jinja')

    yaml_template = yaml.safe_load(template)

    expanded_template = expansion.Expand(
        str(yaml_template), imports)

    result_file = ReadTestFile('jinja_template_with_import_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testJinjaWithInlinedFile(self):
    template = ReadTestFile('jinja_template_with_inlinedfile.yaml')

    imports = {}
    imports['jinja_template_with_inlinedfile.jinja'] = ReadImportFile(
        'jinja_template_with_inlinedfile.jinja')
    imports['helpers/common.jinja'] = ReadImportFile(
        'helpers/common.jinja')
    imports['description_text.txt'] = ReadImportFile('description_text.txt')

    yaml_template = yaml.safe_load(template)

    expanded_template = expansion.Expand(
        str(yaml_template), imports)

    result_file = ReadTestFile('jinja_template_with_inlinedfile_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testJinjaMultilineFile(self):
    template = ReadTestFile('jinja_multilinefile.yaml')

    imports = {}
    imports['jinja_multilinefile.jinja'] = ReadImportFile(
        'jinja_multilinefile.jinja')
    imports['helpers/common.jinja'] = ReadImportFile(
        'helpers/common.jinja')
    imports['multiline.txt'] = ReadImportFile('multiline.txt')

    yaml_template = yaml.safe_load(template)

    expanded_template = expansion.Expand(
        str(yaml_template), imports)

    result_file = ReadTestFile('jinja_multilinefile_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonWithImport(self):
    template = ReadTestFile('python_template_with_import.yaml')

    imports = {}
    imports['python_template_with_import.py'] = ReadImportFile(
        'python_template_with_import.py')
    imports['helpers/common.py'] = ReadImportFile('helpers/common.py')
    imports['helpers/extra/common2.py'] = ReadImportFile(
        'helpers/extra/common2.py')
    imports['helpers/extra'] = ReadImportFile('helpers/extra/__init__.py')

    yaml_template = yaml.safe_load(template)

    expanded_template = expansion.Expand(
        str(yaml_template), imports)

    result_file = ReadTestFile('python_template_with_import_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonWithInlinedFile(self):
    template = ReadTestFile('python_template_with_inlinedfile.yaml')

    imports = {}
    imports['python_template_with_inlinedfile.py'] = ReadImportFile(
        'python_template_with_inlinedfile.py')
    imports['helpers/common.py'] = ReadImportFile('helpers/common.py')
    imports['helpers/extra/common2.py'] = ReadImportFile(
        'helpers/extra/common2.py')
    imports['description_text.txt'] = ReadImportFile('description_text.txt')

    yaml_template = yaml.safe_load(template)

    expanded_template = expansion.Expand(
        str(yaml_template), imports)

    result_file = ReadTestFile(
        'python_template_with_inlinedfile_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonMultilineFile(self):
    template = ReadTestFile('python_multilinefile.yaml')

    imports = {}
    imports['python_multilinefile.py'] = ReadImportFile(
        'python_multilinefile.py')

    imports['helpers/common.py'] = ReadImportFile('helpers/common.py')
    imports['helpers/extra/common2.py'] = ReadImportFile(
        'helpers/extra/common2.py')

    imports['multiline.txt'] = ReadImportFile('multiline.txt')

    yaml_template = yaml.safe_load(template)

    expanded_template = expansion.Expand(
        str(yaml_template), imports)

    result_file = ReadTestFile('python_multilinefile_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testPythonWithEnvironment(self):
    template = ReadTestFile('python_template_with_env.yaml')

    imports = {}
    imports['python_template_with_env.py'] = ReadImportFile(
        'python_template_with_env.py')

    env = {'project': 'my-project'}

    expanded_template = expansion.Expand(
        template, imports, env)

    result_file = ReadTestFile('python_template_with_env_result.yaml')
    self.assertEquals(result_file, expanded_template)

  def testJinjaWithEnvironment(self):
    template = ReadTestFile('jinja_template_with_env.yaml')

    imports = {}
    imports['jinja_template_with_env.jinja'] = ReadImportFile(
        'jinja_template_with_env.jinja')

    env = {'project': 'test-project', 'deployment': 'test-deployment'}

    expanded_template = expansion.Expand(
        template, imports, env)

    result_file = ReadTestFile('jinja_template_with_env_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testMissingNameErrors(self):
    template = 'resources: \n- type: something.jinja\n'

    try:
      expansion.Expand(template, {})
      self.fail('Expansion should fail')
    except expansion.ExpansionError as e:
      self.assertTrue('not have a name' in e.message)

  def testDuplicateNamesErrors(self):
    template = ReadTestFile('duplicate_names.yaml')

    try:
      expansion.Expand(template, {})
      self.fail('Expansion should fail')
    except expansion.ExpansionError as e:
      self.assertTrue(("Resource name 'my_instance' is not unique"
                       " in config.") in e.message)

  def testDuplicateNamesInSubtemplates(self):
    template = ReadTestFile('duplicate_names_in_subtemplates.yaml')

    imports = {}
    imports['duplicate_names_in_subtemplates.jinja'] = ReadImportFile(
        'duplicate_names_in_subtemplates.jinja')

    try:
      expansion.Expand(
          template, imports)
      self.fail('Expansion should fail')
    except expansion.ExpansionError as e:
      self.assertTrue('not unique in duplicate_names_in_subtemplates.jinja'
                      in e.message)

  def testDuplicateNamesMixedLevel(self):
    template = ReadTestFile('duplicate_names_mixed_level.yaml')

    imports = {}
    imports['duplicate_names_B.jinja'] = ReadImportFile(
        'duplicate_names_B.jinja')
    imports['duplicate_names_C.jinja'] = ReadImportFile(
        'duplicate_names_C.jinja')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('duplicate_names_mixed_level_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testDuplicateNamesParentChild(self):
    template = ReadTestFile('duplicate_names_parent_child.yaml')

    imports = {}
    imports['duplicate_names_B.jinja'] = ReadImportFile(
        'duplicate_names_B.jinja')

    expanded_template = expansion.Expand(
        template, imports)

    result_file = ReadTestFile('duplicate_names_parent_child_result.yaml')

    self.assertEquals(result_file, expanded_template)
    # Note, this template will fail in the frontend for duplicate resource names

  def testTemplateReturnsEmpty(self):
    template = ReadTestFile('no_resources.yaml')

    imports = {}
    imports['no_resources.py'] = ReadImportFile(
        'no_resources.py')

    try:
      expansion.Expand(
          template, imports)
      self.fail('Expansion should fail')
    except expansion.ExpansionError as e:
      self.assertIn('Template did not return a \'resources:\' field.',
                    e.message)
      self.assertIn('no_resources.py', e.message)

  def testJinjaDefaultsSchema(self):
    # Loop 100 times to make sure we don't rely on dictionary ordering.
    for unused_x in range(0, 100):
      template = ReadTestFile('jinja_defaults.yaml')

      imports = {}
      imports['jinja_defaults.jinja'] = ReadImportFile(
          'jinja_defaults.jinja')
      imports['jinja_defaults.jinja.schema'] = ReadImportFile(
          'jinja_defaults.jinja.schema')

      expanded_template = expansion.Expand(
          template, imports,
          validate_schema=True)

      result_file = ReadTestFile('jinja_defaults_result.yaml')

      self.assertEquals(result_file, expanded_template)

  def testPythonDefaultsOverrideSchema(self):
    template = ReadTestFile('python_schema.yaml')

    imports = {}
    imports['python_schema.py'] = ReadImportFile('python_schema.py')
    imports['python_schema.py.schema'] = ReadImportFile(
        'python_schema.py.schema')

    env = {'project': 'my-project'}

    expanded_template = expansion.Expand(
        template, imports, env=env,
        validate_schema=True)

    result_file = ReadTestFile('python_schema_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testJinjaMissingRequiredPropertySchema(self):
    template = ReadTestFile('jinja_missing_required.yaml')

    imports = {}
    imports['jinja_missing_required.jinja'] = ReadImportFile(
        'jinja_missing_required.jinja')
    imports['jinja_missing_required.jinja.schema'] = ReadImportFile(
        'jinja_missing_required.jinja.schema')

    try:
      expansion.Expand(
          template, imports,
          validate_schema=True)
      self.fail('Expansion error expected')
    except expansion.ExpansionError as e:
      self.assertIn('Invalid properties', e.message)
      self.assertIn("'important' is a required property", e.message)
      self.assertIn('jinja_missing_required_resource_name', e.message)

  def testJinjaErrorFileMessage(self):
    template = ReadTestFile('jinja_unresolved.yaml')

    imports = {}
    imports['jinja_unresolved.jinja'] = ReadImportFile('jinja_unresolved.jinja')

    try:
      expansion.Expand(
          template, imports,
          validate_schema=False)
      self.fail('Expansion error expected')
    except expansion.ExpansionError as e:
      self.assertIn('jinja_unresolved.jinja', e.message)

  def testJinjaMultipleErrorsSchema(self):
    template = ReadTestFile('jinja_multiple_errors.yaml')

    imports = {}
    imports['jinja_multiple_errors.jinja'] = ReadImportFile(
        'jinja_multiple_errors.jinja')
    imports['jinja_multiple_errors.jinja.schema'] = ReadImportFile(
        'jinja_multiple_errors.jinja.schema')

    try:
      expansion.Expand(
          template, imports,
          validate_schema=True)
      self.fail('Expansion error expected')
    except expansion.ExpansionError as e:
      self.assertIn('Invalid properties', e.message)
      self.assertIn("'a string' is not of type 'integer'", e.message)
      self.assertIn("'d' is not one of ['a', 'b', 'c']", e.message)
      self.assertIn("'longer than 10 chars' is too long", e.message)
      self.assertIn("{'multipleOf': 2} is not allowed for 6", e.message)

  def testPythonBadSchema(self):
    template = ReadTestFile('python_bad_schema.yaml')

    imports = {}
    imports['python_bad_schema.py'] = ReadImportFile(
        'python_bad_schema.py')
    imports['python_bad_schema.py.schema'] = ReadImportFile(
        'python_bad_schema.py.schema')

    try:
      expansion.Expand(
          template, imports,
          validate_schema=True)
      self.fail('Expansion error expected')
    except expansion.ExpansionError as e:
      self.assertIn('Invalid schema', e.message)
      self.assertIn("'int' is not valid under any of the given schemas",
                    e.message)
      self.assertIn("'maximum' is a dependency of u'exclusiveMaximum'",
                    e.message)
      self.assertIn("10 is not of type u'boolean'", e.message)
      self.assertIn("'not a list' is not of type u'array'", e.message)

  def testNoProperties(self):
    template = ReadTestFile('no_properties.yaml')

    imports = {}
    imports['no_properties.py'] = ReadImportFile(
        'no_properties.py')

    expanded_template = expansion.Expand(
        template, imports,
        validate_schema=True)

    result_file = ReadTestFile('no_properties_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testNoPropertiesSchemaDefaults(self):
    template = ReadTestFile('no_properties_schema_defaults.yaml')

    imports = {}
    imports['no_properties_schema_defaults.py'] = ReadImportFile(
        'no_properties_schema_defaults.py')
    imports['no_properties_schema_defaults.py.schema'] = ReadImportFile(
        'no_properties_schema_defaults.py.schema')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True)

    result_file = ReadTestFile('no_properties_schema_defaults_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testNestedTemplateSchema(self):
    template = ReadTestFile('use_helper.yaml')

    imports = {}
    imports['use_helper.jinja'] = ReadImportFile(
        'use_helper.jinja')
    imports['use_helper.jinja.schema'] = ReadImportFile(
        'use_helper.jinja.schema')
    imports['helper.jinja'] = ReadImportFile(
        'helper.jinja')
    imports['helper.jinja.schema'] = ReadImportFile(
        'helper.jinja.schema')

    expanded_template = expansion.Expand(
        template, imports,
        validate_schema=True)

    result_file = ReadTestFile('use_helper_result.yaml')

    self.assertEquals(result_file, expanded_template)

  # Output Tests

  def testSimpleOutput(self):
    template = ReadTestFile('outputs/simple.yaml')

    expanded_template = expansion.Expand(
        template, {}, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/simple_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testSimpleTemplateOutput(self):
    template = ReadTestFile('outputs/template.yaml')

    imports = {}
    imports['simple.jinja'] = ReadImportFile(
        'outputs/simple.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/template_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testChainOutput(self):
    template = ReadTestFile('outputs/chain_outputs.yaml')

    imports = {}
    imports['simple.jinja'] = ReadImportFile(
        'outputs/simple.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/chain_outputs_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testChainMultiple(self):
    template = ReadTestFile('outputs/chain_multiple.yaml')

    imports = {}
    imports['simple.jinja'] = ReadImportFile('outputs/simple.jinja')
    imports['one_simple.jinja'] = ReadImportFile('outputs/one_simple.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/chain_multiple_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testConsumeOutput(self):
    template = ReadTestFile('outputs/consume_output.yaml')

    imports = {}
    imports['simple.jinja'] = ReadImportFile('outputs/simple.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/consume_output_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testConsumeMultiple(self):
    template = ReadTestFile('outputs/consume_multiple.yaml')

    imports = {}
    imports['simple.jinja'] = ReadImportFile('outputs/simple.jinja')
    imports['one_consume.jinja'] = ReadImportFile('outputs/one_consume.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/consume_multiple_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testConsumeListOutput(self):
    template = ReadTestFile('outputs/list_output.yaml')

    imports = {}
    imports['list_output.jinja'] = ReadImportFile('outputs/list_output.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/list_output_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testSimpleUpDown(self):
    template = ReadTestFile('outputs/simple_up_down.yaml')

    imports = {}
    imports['instance_builder.jinja'] = ReadImportFile(
        'outputs/instance_builder.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/simple_up_down_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testUpDown(self):
    template = ReadTestFile('outputs/up_down.yaml')

    imports = {}
    imports['frontend.jinja'] = ReadImportFile('outputs/frontend.jinja')
    imports['backend.jinja'] = ReadImportFile('outputs/backend.jinja')
    imports['instance_builder.jinja'] = ReadImportFile(
        'outputs/instance_builder.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/up_down_result.yaml')

    self.assertEquals(result_file, expanded_template)

  def testUpDownWithOutputsOff(self):
    template = ReadTestFile('outputs/up_down.yaml')

    imports = {}
    imports['frontend.jinja'] = ReadImportFile('outputs/frontend.jinja')
    imports['backend.jinja'] = ReadImportFile('outputs/backend.jinja')
    imports['instance_builder.jinja'] = ReadImportFile(
        'outputs/instance_builder.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=False)

    result_file = ReadTestFile('outputs/up_down_result_off.yaml')

    self.assertEquals(result_file, expanded_template)

  def testConditionalDoesntWork(self):
    """Verifies that conditionals on references don't work.

    That is, you can't output 2 then use that value in another template to
    create 2 instances.
    """
    template = ReadTestFile('outputs/conditional.yaml')

    imports = {}
    imports['conditional.jinja'] = ReadImportFile('outputs/conditional.jinja')
    imports['output_one.jinja'] = ReadImportFile('outputs/output_one.jinja')

    expanded_template = expansion.Expand(
        template, imports, validate_schema=True, outputs=True)

    result_file = ReadTestFile('outputs/conditional_result.yaml')

    self.assertEquals(result_file, expanded_template)

if __name__ == '__main__':
  unittest.main()
