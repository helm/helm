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

"""Validation of Template properties for deployment manager v2."""

import jsonschema
import yaml

import references
import schema_validation_utils


IMPORTS = "imports"
PROPERTIES = "properties"


# This validator will set default values in properties.
# This does not return a complete set of errors; use only for setting defaults.
# Pass this object a schema to get a validator for that schema.
DEFAULT_SETTER = schema_validation_utils.ExtendWithDefault(
    jsonschema.Draft4Validator)

# This is a regular validator, use after using the DEFAULT_SETTER
# Pass this object a schema to get a validator for that schema.
VALIDATOR = jsonschema.Draft4Validator

# This is a validator using the default Draft4 metaschema,
# use it to validate user schemas.
SCHEMA_VALIDATOR = jsonschema.Draft4Validator(
    jsonschema.Draft4Validator.META_SCHEMA)

# JsonSchema to be used to validate the user's "imports:" section
IMPORT_SCHEMA = """
  properties:
    imports:
      type: array
      items:
        type: object
        required:
          - path
        properties:
          path:
            type: string
          name:
            type: string
        additionalProperties: false
      uniqueItems: true
"""
# Validator to be used against the "imports:" section of a schema
IMPORT_SCHEMA_VALIDATOR = jsonschema.Draft4Validator(
    yaml.safe_load(IMPORT_SCHEMA))


def _FilterReferences(error_generator):
  for error in error_generator:
    if not references.HasReference(str(error.instance)):
      yield error


def _ValidateSchema(schema, validating_imports, schema_name, template_name):
  """Validate that the passed in schema file is correctly formatted.

  Args:
    schema: contents of the schema file
    validating_imports: boolean, if we should validate the 'imports'
        section of the schema
    schema_name: name of the schema file to validate
    template_name: name of the template whose properties are being validated

  Raises:
    ValidationErrors: A list of ValidationError errors that occured when
        validating the schema file
  """
  schema_errors = []

  # Validate the syntax of the optional "imports:" section of the schema
  if validating_imports:
    schema_errors.extend(IMPORT_SCHEMA_VALIDATOR.iter_errors(schema))

  # Validate the syntax of the jsonSchema section of the schema
  try:
    schema_errors.extend(SCHEMA_VALIDATOR.iter_errors(schema))
  except jsonschema.RefResolutionError as e:
    # Calls to iter_errors could throw a RefResolution exception
    raise ValidationErrors(schema_name, template_name,
                           [e], is_schema_error=True)

  if schema_errors:
    raise ValidationErrors(schema_name, template_name,
                           schema_errors, is_schema_error=True)


def Validate(properties, schema_name, template_name, imports):
  """Given a set of properties, validates it against the given schema.

  Args:
    properties: dict, the properties to be validated
    schema_name: name of the schema file to validate
    template_name: name of the template whose properties are being validated
    imports: the map of imported files names to file contents

  Returns:
    Dict containing the validated properties, with defaults filled in

  Raises:
    ValidationErrors: A list of ValidationError errors that occurred when
        validating the properties and schema,
        or if the schema file was not found
  """
  if schema_name not in imports:
    raise ValidationErrors(schema_name, template_name,
                           ["Could not find schema file '%s'." % schema_name])

  raw_schema = imports[schema_name]

  if properties is None:
    properties = {}

  schema = yaml.safe_load(raw_schema)

  # If the schema is empty, do nothing.
  if not schema:
    return properties

  validating_imports = IMPORTS in schema and schema[IMPORTS]

   # If this doesn't raise any exceptions, we can assume we have a valid schema
  _ValidateSchema(schema, validating_imports, schema_name, template_name)

  errors = []

  # Validate that all files specified as "imports:" were included
  if validating_imports:
    # We have already validated that "imports:"
    # is a list of unique "path/name" maps
    for import_object in schema[IMPORTS]:
      if "name" in import_object:
        import_name = import_object["name"]
      else:
        import_name = import_object["path"]

      if import_name not in imports:
        errors.append(("File '%s' requested in schema '%s' "
                       "but not included with imports."
                       % (import_name, schema_name)))

  try:
    # This code block uses DEFAULT_SETTER and VALIDATOR for two very
    # different purposes.
    # DEFAULT_SETTER is based on JSONSchema 4, but uses modified validators:
    # - The 'required' validator does nothing
    # - The 'properties' validator sets default values on user properties
    # With these changes, the validator does not report errors correctly.
    #
    # So, we do error reporting in two steps:
    # 1) Use DEFAULT_SETTER to set default values in the user's properties
    # 2) Use the unmodified VALIDATOR to report all of the errors

    # Calling iter_errors mutates properties in place, adding default values.
    # You must call list()! This is a generator, not a function!
    list(DEFAULT_SETTER(schema).iter_errors(properties))

    # Now that we have default values, validate the properties
    errors.extend(_FilterReferences(VALIDATOR(schema).iter_errors(properties)))

    if errors:
      raise ValidationErrors(schema_name, template_name, errors)
  except jsonschema.RefResolutionError as e:
    # Calls to iter_errors could throw a RefResolution exception
    raise ValidationErrors(schema_name, template_name,
                           [e], is_schema_error=True)
  except TypeError as e:
    raise ValidationErrors(
        schema_name, template_name,
        [e, "Perhaps you forgot to put 'quotes' around your reference."],
        is_schema_error=True)

  return properties


class ValidationErrors(Exception):
  """Exception raised for errors during validation process.

  The errors could have occured either in the schema xor in the properties

  Attributes:
    is_schema_error: Boolean, either an invalid schema, or invalid properties
    errors: List of ValidationError type objects
  """

  def BuildMessage(self):
    """Builds a human readable message from a list of jsonschema errors.

    Returns:
      A string in a human readable message format.
    """

    if self.is_schema_error:
      message = "Invalid schema '%s':\n" % self.schema_name
    else:
      message = "Invalid properties for '%s':\n" % self.template_name

    for error in self.errors:
      if isinstance(error, jsonschema.exceptions.ValidationError):
        error_message = error.message
        location = list(error.path)
        if location and len(location):
          error_message += " at " + str(location)
        # If location is empty the error happened at the root of the schema
      else:
        error_message = str(error)

      message += error_message + "\n"

    return message

  def __init__(self, schema_name, template_name, errors, is_schema_error=False):
    self.schema_name = schema_name
    self.template_name = template_name
    self.errors = errors
    self.is_schema_error = is_schema_error
    self.message = self.BuildMessage()
    super(ValidationErrors, self).__init__(self.message)
