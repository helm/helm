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

import os
import unittest
import schema_validation
import yaml

INVALID_PROPERTIES = "Invalid properties for 'template.py'"


def GetFilePath():
  """Find our source and data files."""
  return  os.path.dirname(os.path.abspath(__file__))


def ReadTestFile(filename):
  """Returns contents of a file from the testdata/ directory."""

  full_path = os.path.join(GetFilePath(), '..', 'test', 'schemas', filename)
  return open(full_path, 'r').read()


def RawValidate(raw_properties, schema_name, raw_schema):
  return ImportsRawValidate(raw_properties, schema_name,
                            {schema_name: raw_schema})


def ImportsRawValidate(raw_properties, schema_name, import_map):
  """Takes raw properties, calls validate and returns yaml properties."""
  properties = yaml.safe_load(raw_properties)
  return schema_validation.Validate(properties, schema_name, 'template.py',
                                    import_map)


class SchemaValidationTest(unittest.TestCase):
  """Tests of the schema portion of the template expansion library."""

  def testDefaults(self):
    schema_name = 'defaults.jinja.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    expected_properties = """
      alpha: alpha
      one: 1
    """
    self.assertEqual(yaml.safe_load(expected_properties),
                     RawValidate(empty_properties, schema_name, schema))

  def testNestedDefaults(self):
    schema_name = 'nested_defaults.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      zone: us-central1-a
      disks:
        - name: backup       # diskType and sizeGb set by default
        - name: cache        # sizeGb set by default
          diskType: pd-ssd
        - name: data         # Nothing set by default
          diskType: pd-ssd
          sizeGb: 150
        - name: swap         # diskType set by default
          sizeGb: 200
    """
    expected_properties = """
      zone: us-central1-a
      disks:
        - sizeGb: 100
          diskType: pd-standard
          name: backup
        - sizeGb: 100
          diskType: pd-ssd
          name: cache
        - sizeGb: 150
          diskType: pd-ssd
          name: data
        - sizeGb: 200
          diskType: pd-standard
          name: swap
    """
    self.assertEqual(yaml.safe_load(expected_properties),
                     RawValidate(properties, schema_name, schema))

  def testNestedRefDefaults(self):
    schema_name = 'ref_nested_defaults.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      zone: us-central1-a
      disks:
        - name: backup       # diskType and sizeGb set by default
        - name: cache        # sizeGb set by default
          diskType: pd-ssd
        - name: data         # Nothing set by default
          diskType: pd-ssd
          sizeGb: 150
        - name: swap         # diskType set by default
          sizeGb: 200
    """
    expected_properties = """
      zone: us-central1-a
      disks:
        - sizeGb: 100
          diskType: pd-standard
          name: backup
        - sizeGb: 100
          diskType: pd-ssd
          name: cache
        - sizeGb: 150
          diskType: pd-ssd
          name: data
        - sizeGb: 200
          diskType: pd-standard
          name: swap
    """
    self.assertEqual(yaml.safe_load(expected_properties),
                     RawValidate(properties, schema_name, schema))

  def testInvalidDefault(self):
    schema_name = 'invalid_default.jinja.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn(INVALID_PROPERTIES, e.message)
      self.assertIn("'string' is not of type 'integer' at ['number']",
                    e.message)

  def testRequiredDefault(self):
    schema_name = 'required_default.jinja.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    expected_properties = """
      name: my_name
    """
    self.assertEqual(yaml.safe_load(expected_properties),
                     RawValidate(empty_properties, schema_name, schema))

  def testRequiredDefaultReference(self):
    schema_name = 'req_default_ref.py.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''

    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn(INVALID_PROPERTIES, e.message)
      self.assertIn("'my_name' is not of type 'integer' at ['number']",
                    e.message)

  def testDefaultReference(self):
    schema_name = 'default_ref.jinja.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    expected_properties = 'number: 1'

    self.assertEqual(yaml.safe_load(expected_properties),
                     RawValidate(empty_properties, schema_name, schema))

  def testMissingQuoteInReference(self):
    schema_name = 'missing_quote.py.schema'
    schema = ReadTestFile(schema_name)
    properties = 'number: 1'

    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(2, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("type 'NoneType' is not iterable", e.message)
      self.assertIn('around your reference', e.message)

  def testRequiredPropertyMissing(self):
    schema_name = 'required.jinja.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn(INVALID_PROPERTIES, e.message)
      self.assertIn("'name' is a required property", e.errors[0].message)

  def testRequiredPropertyValid(self):
    schema_name = 'required.jinja.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      name: my-name
    """
    self.assertEqual(yaml.safe_load(properties),
                     RawValidate(properties, schema_name, schema))

  def testMultipleErrors(self):
    schema_name = 'defaults.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      one: not a number
      alpha: 12345
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(2, len(e.errors))
      self.assertIn(INVALID_PROPERTIES, e.message)
      self.assertIn("'not a number' is not of type 'integer' at ['one']",
                    e.message)
      self.assertIn("12345 is not of type 'string' at ['alpha']", e.message)

  def testNumbersValid(self):
    schema_name = 'numbers.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      minimum0: 0
      exclusiveMin0: 1
      maximum10: 10
      exclusiveMax10: 9
      even: 20
      odd: 21
    """
    self.assertEquals(yaml.safe_load(properties),
                      RawValidate(properties, schema_name, schema))

  def testNumbersInvalid(self):
    schema_name = 'numbers.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      minimum0: -1
      exclusiveMin0: 0
      maximum10: 11
      exclusiveMax10: 10
      even: 21
      odd: 20
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(6, len(e.errors))
      self.assertIn(INVALID_PROPERTIES, e.message)
      self.assertIn("-1 is less than the minimum of 0 at ['minimum0']",
                    e.message)
      self.assertIn(('0 is less than or equal to the minimum of 0'
                     " at ['exclusiveMin0']"), e.message)
      self.assertIn("11 is greater than the maximum of 10 at ['maximum10']",
                    e.message)
      self.assertIn(('10 is greater than or equal to the maximum of 10'
                     " at ['exclusiveMax10']"), e.message)
      self.assertIn("21 is not a multiple of 2 at ['even']", e.message)
      self.assertIn("{'multipleOf': 2} is not allowed for 20 at ['odd']",
                    e.message)

  def testReference(self):
    schema_name = 'reference.jinja.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      odd: 6
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn('even', e.message)
      self.assertIn('is not allowed for 6', e.message)

  def testBadSchema(self):
    schema_name = 'bad.jinja.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(2, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("u'minimum' is a dependency of u'exclusiveMinimum'",
                    e.message)
      self.assertIn("0 is not of type u'boolean'", e.message)

  def testInvalidReference(self):
    schema_name = 'invalid_reference.py.schema'
    schema = ReadTestFile(schema_name)
    properties = 'odd: 1'
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn('Unresolvable JSON pointer', e.message)

  def testInvalidReferenceInSchema(self):
    schema_name = 'invalid_reference_schema.py.schema'
    schema = ReadTestFile(schema_name)
    empty_properties = ''
    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn('Unresolvable JSON pointer', e.message)

  def testMetadata(self):
    schema_name = 'metadata.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      one: 2
      alpha: beta
    """
    self.assertEquals(yaml.safe_load(properties),
                      RawValidate(properties, schema_name, schema))

  def testInvalidInput(self):
    schema_name = 'schema'
    schema = """
      info:
        title: Invalid Input
      properties: invalid
    """
    properties = """
      one: 2
      alpha: beta
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("'invalid' is not of type u'object'", e.message)

  def testPattern(self):
    schema_name = 'schema'
    schema = r"""
      properties:
        bad-zone:
          pattern: \w+-\w+-\w+
        zone:
          pattern: \w+-\w+-\w+
    """
    properties = """
      bad-zone: abc
      zone: us-central1-a
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn('Invalid properties', e.message)
      self.assertIn("'abc' does not match", e.message)
      self.assertIn('bad-zone', e.message)

  def testUniqueItems(self):
    schema_name = 'schema'
    schema = """
      properties:
        bad-list:
          type: array
          uniqueItems: true
        list:
          type: array
          uniqueItems: true
    """
    properties = """
      bad-list:
      - a
      - b
      - a
      list:
      - a
      - b
      - c
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn('Invalid properties', e.message)
      self.assertIn('has non-unique elements', e.message)
      self.assertIn('bad-list', e.message)

  def testUniqueItemsOnString(self):
    schema_name = 'schema'
    schema = """
      properties:
        ok-string:
          type: string
          uniqueItems: true
        string:
          type: string
          uniqueItems: true
    """
    properties = """
      ok-string: aaa
      string: abc
    """
    self.assertEquals(yaml.safe_load(properties),
                      RawValidate(properties, schema_name, schema))

  def testRequiredTopLevel(self):
    schema_name = 'schema'
    schema = """
      info:
        title: Invalid Input
      required:
        - name
    """
    properties = """
      one: 2
      alpha: beta
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn(INVALID_PROPERTIES, e.message)
      self.assertIn("'name' is a required property", e.message)

  def testEmptySchemaProperties(self):
    schema_name = 'schema'
    schema = """
      info:
        title: Empty Input
      properties:
    """
    properties = """
      one: 2
      alpha: beta
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("None is not of type u'object' at [u'properties']",
                    e.message)

  def testNoInput(self):
    schema = """
      info:
        title: No other sections
    """
    properties = """
      one: 2
      alpha: beta
    """
    self.assertEquals(yaml.safe_load(properties),
                      RawValidate(properties, 'schema', schema))

  def testEmptySchema(self):
    schema = ''
    properties = """
      one: 2
      alpha: beta
    """
    self.assertEquals(yaml.safe_load(properties),
                      RawValidate(properties, 'schema', schema))

  def testImportPathSchema(self):
    schema = """
      imports:
        - path: a
        - path: path/to/b
          name: b
    """
    properties = """
      one: 2
      alpha: beta
    """

    import_map = {'schema': schema,
                  'a': '',
                  'b': ''}

    self.assertEquals(yaml.safe_load(properties),
                      ImportsRawValidate(properties, 'schema', import_map))

  def testImportSchemaMissing(self):
    schema = ''
    empty_properties = ''

    try:
      properties = yaml.safe_load(empty_properties)
      schema_validation.Validate(properties, 'schema', 'template',
                                 {'wrong_name': schema})
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Could not find schema file 'schema'", e.message)

  def testImportsMalformedNotAList(self):
    schema_name = 'schema'
    schema = """
      imports: not-a-list
      """
    empty_properties = ''

    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("is not of type 'array' at ['imports']", e.message)

  def testImportsMalformedMissingPath(self):
    schema_name = 'schema'
    schema = """
      imports:
        - name: no_path.yaml
      """
    empty_properties = ''

    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("'path' is a required property", e.message)

  def testImportsMalformedNonunique(self):
    schema_name = 'schema'
    schema = """
      imports:
        - path: a.yaml
          name: a
        - path: a.yaml
          name: a
      """
    empty_properties = ''

    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn('non-unique elements', e.message)

  def testImportsMalformedAdditionalProperties(self):
    schema_name = 'schema'
    schema = """
      imports:
        - path: a.yaml
          gnome: a
      """
    empty_properties = ''

    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn('Additional properties are not allowed'
                    " ('gnome' was unexpected)", e.message)

  def testImportAndInputErrors(self):
    schema = """
      imports:
        - path: file
      required:
        - name
      """
    empty_properties = ''

    try:
      RawValidate(empty_properties, 'schema', schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(2, len(e.errors))
      self.assertIn("'file' requested in schema 'schema'", e.message)
      self.assertIn("'name' is a required property", e.message)

  def testImportAndInputSchemaErrors(self):
    schema_name = 'schema'
    schema = """
      imports: not-a-list
      required: not-a-list
      """
    empty_properties = ''

    try:
      RawValidate(empty_properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(2, len(e.errors))
      self.assertIn("Invalid schema '%s'" % schema_name, e.message)
      self.assertIn("is not of type 'array' at ['imports']", e.message)
      self.assertIn("is not of type u'array' at [u'required']", e.message)

  def testNoValidateReference_Simple(self):
    schema = """
      properties:
        number:
          type: integer
    """
    properties = """
      number: $(ref.foo.size)
    """
    self.assertEquals(yaml.safe_load(properties),
                      RawValidate(properties, 'schema', schema))

  def testNoValidateReference_OtherErrorNotFiltered(self):
    schema = """
      properties:
        number:
          type: integer
        also-number:
          type: integer
    """
    properties = """
      number: $(ref.foo.size)
      also-number: not a number
    """

    try:
      RawValidate(properties, 'schema', schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEquals(1, len(e.errors))

  def testNoValidateReference_NestedError(self):
    schema_name = 'nested_objects.py.schema'
    schema = ReadTestFile(schema_name)
    properties = """
      one:
        name: my-database
        size: $(ref.other-database.size)
      two:
        name: other-database
        size: really big
    """
    try:
      RawValidate(properties, schema_name, schema)
      self.fail('Validation should fail')
    except schema_validation.ValidationErrors as e:
      self.assertEqual(1, len(e.errors))
      self.assertIn("is not of type 'integer' at ['two', 'size']", e.message)

if __name__ == '__main__':
  unittest.main()
