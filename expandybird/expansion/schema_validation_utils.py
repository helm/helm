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

"""Helper functions for Schema Validation."""

import jsonschema

DEFAULT = "default"
PROPERTIES = "properties"
REF = "$ref"
REQUIRED = "required"


def OnlyValidateProperties(validator_class):
  """Takes a validator and makes it process only the 'properties' top level.

  Args:
    validator_class: A class to add a new validator to

  Returns:
    A validator_class that will validate properties against things
    under the top level "properties" field
  """

  def PropertiesValidator(unused_validator, inputs, instance, schema):
    if inputs is None:
      inputs = {}
    for error in validator_class(schema).iter_errors(instance, inputs):
      yield error

  # This makes sure the only keyword jsonschema will validate is 'properties'
  new_validators = ClearValidatorMap(validator_class.VALIDATORS)
  new_validators.update({PROPERTIES: PropertiesValidator})

  return jsonschema.validators.extend(
      validator_class, new_validators)


def ExtendWithDefault(validator_class):
  """Takes a validator and makes it set default values on properties.

  Args:
    validator_class: A class to add our overridden validators to

  Returns:
    A validator_class that will set default values and ignore required fields
  """

  def SetDefaultsInProperties(validator, properties, instance, unused_schema):
    if properties is None:
      properties = {}
    SetDefaults(validator, properties, instance)

  return jsonschema.validators.extend(
      validator_class, {PROPERTIES: SetDefaultsInProperties,
                        REQUIRED: IgnoreKeyword})


def SetDefaults(validator, properties, instance):
  """Populate the default values of properties.

  Args:
    validator: A generator that validates the "properties" keyword
    properties: User properties on which to set defaults
    instance: Piece of user schema containing "properties"
  """
  if not properties:
    return

  for dm_property, subschema in properties.iteritems():
    # If the property already has a value, we don't need it's default
    if dm_property in instance:
      return

    # The ordering of these conditions assumes that '$ref' blocks override
    # all other schema info, which is what the jsonschema library assumes.

    # If the subschema has a reference,
    # see if that reference defines a 'default' value
    if REF in subschema:
      out = ResolveReferencedDefault(validator, subschema[REF])
      instance.setdefault(dm_property, out)
    # Otherwise, see if the subschema has a 'default' value
    elif DEFAULT in subschema:
      instance.setdefault(dm_property, subschema[DEFAULT])


def ResolveReferencedDefault(validator, ref):
  """Resolves a reference, and returns any default value it defines.

  Args:
    validator: A generator the validates the "$ref" keyword
    ref: The target of the "$ref" keyword

  Returns:
    The value of the 'default' field found in the referenced schema, or None
  """
  with validator.resolver.resolving(ref) as resolved:
    if DEFAULT in resolved:
      return resolved[DEFAULT]


def ClearValidatorMap(validators):
  """Remaps all JsonSchema validators to make them do nothing."""
  ignore_validators = {}
  for keyword in validators:
    ignore_validators.update({keyword: IgnoreKeyword})
  return ignore_validators


def IgnoreKeyword(
    unused_validator, unused_required, unused_instance, unused_schema):
  """Validator for JsonSchema that does nothing."""
  pass
