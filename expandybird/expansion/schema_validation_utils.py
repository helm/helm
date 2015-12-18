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

"""Helper functions for Schema Validation."""

import jsonschema

DEFAULT = 'default'
PROPERTIES = 'properties'
REF = '$ref'
REQUIRED = 'required'


def ExtendWithDefault(validator_class):
  """Takes a validator and makes it set default values on properties.

  Args:
    validator_class: A class to add our overridden validators to

  Returns:
    A validator_class that will set default values and ignore required fields
  """
  validate_properties = validator_class.VALIDATORS['properties']

  def SetDefaultsInProperties(validator, user_schema, user_properties,
                              parent_schema):
    SetDefaults(validator, user_schema or {}, user_properties, parent_schema,
                validate_properties)

  return jsonschema.validators.extend(
      validator_class, {PROPERTIES: SetDefaultsInProperties,
                        REQUIRED: IgnoreKeyword})


def SetDefaults(validator, user_schema, user_properties, parent_schema,
                validate_properties):
  """Populate the default values of properties.

  Args:
    validator: A generator that validates the "properties" keyword of the schema
    user_schema: Schema which might define defaults, might be a nested part of
      the entire schema file.
    user_properties: User provided values which we are setting defaults on
    parent_schema: Schema object that contains the schema being evaluated on
      this pass, user_schema.
    validate_properties: Validator function, called recursively.
  """

  for schema_property, subschema in user_schema.iteritems():
    # The ordering of these conditions assumes that '$ref' blocks override
    # all other schema info, which is what the jsonschema library assumes.

    # If the subschema has a reference,
    # see if that reference defines a 'default' value
    if REF in subschema:
      out = ResolveReferencedDefault(validator, subschema[REF])
      user_properties.setdefault(schema_property, out)
    # Otherwise, see if the subschema has a 'default' value
    elif DEFAULT in subschema:
      user_properties.setdefault(schema_property, subschema[DEFAULT])

  # Recursively apply defaults. This is a generator, so we must wrap with list()
  list(validate_properties(validator, user_schema,
                           user_properties, parent_schema))


def ResolveReferencedDefault(validator, ref):
  """Resolves a reference, and returns any default value it defines.

  Args:
    validator: A generator that validates the "$ref" keyword
    ref: The target of the "$ref" keyword

  Returns:
    The value of the 'default' field found in the referenced schema, or None
  """
  with validator.resolver.resolving(ref) as resolved:
    if DEFAULT in resolved:
      return resolved[DEFAULT]


def IgnoreKeyword(
    unused_validator, unused_required, unused_instance, unused_schema):
  """Validator for JsonSchema that does nothing."""
  pass
