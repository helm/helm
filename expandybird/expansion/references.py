######################################################################
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
######################################################################
"""Util to handle references during expansion."""

import jsonpath
import re


# Generally the regex is for this pattern: $(ref.NAME.PATH)
# NOTE: This will find matches when a reference is present, but the 2nd group
# does NOT necessarily match the 'path' in the reference.
REF_PATTERN = re.compile(r'\$\(ref\.(.*?)\.(.*)\)')

# Regex for the beginning of a reference.
REF_PREFIX_PATTERN = re.compile(r'\$\(ref\.')


def HasReference(string):
  """Returns true if the string contains a reference.

  We are only looking for the first part of a reference, from there we assume
  the user meant to use a reference, and will fail at a later point if no
  complete reference is found.

  Args:
    string: The string to parse to see if it contains  '$(ref.'.

  Returns:
    True if there is at least one reference inside the string.

  Raises:
      ExpansionReferenceError: If we see '$(ref.' but do not find a complete
          reference, we raise this error.
  """
  return ReferenceMatcher(string).FindReference()


def _BuildReference(name, path):
  """Takes name and path and returns '$(ref.name.path)'.

  Args:
    name: String, name of the resource being referenced.
    path: String, jsonPath to the value being referenced.

  Returns:
    String, the complete reference string in the expected format.
  """
  return '$(ref.%s.%s)' % (name, path)


def _ExtractWithJsonPath(ref_obj, name, path, raise_exception=True):
  """Given a path and an object, use jsonpath to extract the value.

  Args:
    ref_obj: Dict obj, the thing being referenced.
    name: Name of the resource being referenced, for the error message.
    path: Path to follow on the ref_obj to get the desired value.
    raise_exception: boolean, set to False and this function will return None
        if no value was found at the path instead of throwing an exception.

  Returns:
    Either the value found at the path, or if raise_exception=False, None.

  Raises:
    ExpansionReferenceError: if there was a error when evaluation the path, or
        no value was found.
  """
  try:
    result = jsonpath.jsonpath(ref_obj, path)
    #  jsonpath should either return a list or False
    if not isinstance(result, list):
      if raise_exception:
        raise Exception('No value found.')
      return None
    # If jsonpath returns a list of a single item, it is lying.
    # It really found that item, and put it in a list.
    # If the reference is to a list, jsonpath will return a list of a list.
    if len(result) == 1:
      return result[0]
    # But if jsonpath returns a list with multiple elements, the path involved
    # wildcards, and the user expects a list of results.
    return result
  # This will usually be an IndexOutOfBounds error, but not always...
  except Exception as e:  # pylint: disable=broad-except
    if raise_exception:
      raise ExpansionReferenceError(_BuildReference(name, path), e.message)
    return None


def PopulateReferences(node, output_map):
  return _TraverseNode(node, None, output_map)


def _TraverseNode(node, list_references=None, output_map=None):
  """Traverse a dict/list/element to find and resolve references.

  Same as DocumentReferenceHandler.java. This function traverses a dictionary
  that can contain dicts, lists, and elements.

  Args:
    node: Object to traverse: dict, list, or string
    list_references: If present, we will append all references we find to it.
        References will be in the form of a (name, path) tuple.
    output_map: Map of resource name to map of output object name to output
        value. If present, we will replace references with the values they
        reference.

  Returns:
    The node. If we were provided an output_map, we'll replace references with
    the value they reference.
  """
  if isinstance(node, dict):
    for key in node:
      node[key] = _TraverseNode(node[key], list_references, output_map)
  elif isinstance(node, list):
    for i in range(len(node)):
      node[i] = _TraverseNode(node[i], list_references, output_map)
  elif isinstance(node, str):
    rm = ReferenceMatcher(node)
    while rm.FindReference():
      if list_references is not None:
        list_references.append((rm.name, rm.path))
      if output_map is not None:
        if rm.name not in output_map:
          continue
        # It is possible that an output value and real resource share a name.
        # In this case, a path could be valid for the real resource but not the
        # output. So we don't fail, we let the reference exists as is.
        value = _ExtractWithJsonPath(output_map[rm.name], rm.name, rm.path,
                                     raise_exception=True)
        if value is not None:
          node = node.replace(_BuildReference(rm.name, rm.path), value)
  return node


class ReferenceMatcher(object):
  """Finds and extracts references from strings.

  Same as DocumentReferenceHandler.java. This class is meant to be similar to
  the re2 matcher class, but specifically tuned for references.
  """
  content = None
  name = None
  path = None

  def __init__(self, content):
    self.content = content

  def FindReference(self):
    """Returns True if the string contains a reference and saves it.

    Returns:
      True if the content still contains a reference. If so, it also updates the
      name and path values to that of the most recently found reference. At the
      same time it moves the pointer in the content forward to be ready to find
      the next reference.

    Raises:
      ExpansionReferenceError: If we see '$(ref.' but do not find a complete
          reference, we raise this error.
    """

    # First see if the content contains '$(ref.'
    if not REF_PREFIX_PATTERN.search(self.content):
      return False

    # If so, then we say there is a reference here.
    # Next make sure we find NAME and some PATH with the close paren
    match = REF_PATTERN.search(self.content)
    if not match:
      # Has '$(ref.' but not '$(ref.NAME.PATH)'
      raise ExpansionReferenceError(self.content, 'Malformed reference.')

    # The regex matcher can only tell us that a complete reference exists.
    # We need to count parentheses to find the end of the reference.
    # Consider "$(ref.NAME.path())())" which is a string containing a reference
    # and ending with "())"

    open_group = 1  # Count the first '(' in '$(ref...'
    end_ref = 0     # To hold the position of the end of the reference
    end_name = match.end(1)  # The position of the end of the name

    # Iterate through the path until we find the matching close paren to the
    # open paren that started the reference.
    for i in xrange(end_name, len(self.content)):
      c = self.content[i]
      if c == '(':
        open_group += 1
      elif c == ')':
        open_group -= 1

      # Once we have matched all of our open parens, we have found the end.
      if open_group == 0:
        end_ref = i
        break

    if open_group != 0:
      # There are unmatched parens.
      raise ExpansionReferenceError(self.content, 'Malformed reference.')

    # Save the name
    self.name = match.group(1)
    # Skip the period after name, and save the path
    self.path = self.content[end_name + 1: end_ref]

    # Move the content forward to be ready to find the next reference
    self.content = self.content[end_ref:]

    return True


class ExpansionReferenceError(Exception):
  """Exception raised when jsonPath cannot find the referenced value.

  Attributes:
    reference: the reference processed that results in the error
    message: the detailed message of the error
  """

  def __init__(self, reference, message):
    self.reference = reference
    self.message = message + ' Reference: ' + str(reference)
    super(ExpansionReferenceError, self).__init__(self.message)
