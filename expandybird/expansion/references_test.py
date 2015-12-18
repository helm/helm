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
"""Basic unit tests for template expansion references library."""

import unittest
from references import _ExtractWithJsonPath
from references import _TraverseNode
from references import ExpansionReferenceError
from references import HasReference
from references import ReferenceMatcher


class ReferencesTest(unittest.TestCase):

  # Tests for HasReference

  def testBasicReference(self):
    self.assertTrue(HasReference('$(ref.name.path)'))

  def testEmbeddedReference(self):
    self.assertTrue(HasReference('contains reference $(ref.name.path) EOM'))

  def testComplexPath(self):
    self.assertTrue(HasReference('$(ref.name.path[0].to().very.cool["thing"])'))

  def testComplexName(self):
    self.assertTrue(HasReference('$(ref.name-is-superCool.path)'))

  def testMissingGroupClose(self):
    try:
      HasReference('almost a reference $(ref.name.path')
      self.Fail('Expected Reference exception')
    except ExpansionReferenceError as e:
      self.assertTrue('Malformed' in e.message)
      self.assertTrue('$(ref.name.path' in e.message)

  def testMissingGroupOpen(self):
    # Not close enough to find a match
    self.assertFalse(HasReference('almost a reference $ref.name.path)'))

  def testMissingPath(self):
    try:
      self.assertTrue(HasReference('almost a reference $(ref.name)'))
      self.Fail('Expected Reference exception')
    except ExpansionReferenceError as e:
      self.assertTrue('Malformed' in e.message)
      self.assertTrue('$(ref.name)' in e.message)

  def testUnmatchedParens(self):
    try:
      self.assertTrue(HasReference('almost a reference $(ref.name.path()'))
      self.Fail('Expected Reference exception')
    except ExpansionReferenceError as e:
      self.assertTrue('Malformed' in e.message)
      self.assertTrue('$(ref.name.path()' in e.message)

  def testMissingRef(self):
    self.assertFalse(HasReference('almost a reference $(name.path)'))

  # Test for ReferenceMatcher

  def testMatchBasic(self):
    matcher = ReferenceMatcher('$(ref.NAME.PATH)')
    self.assertTrue(matcher.FindReference())
    self.assertEquals(matcher.name, 'NAME')
    self.assertEquals(matcher.path, 'PATH')
    self.assertFalse(matcher.FindReference())

  def testMatchComplexPath(self):
    matcher = ReferenceMatcher('inside a $(ref.NAME.path[?(@.price<10)].val)!')
    self.assertTrue(matcher.FindReference())
    self.assertEquals(matcher.name, 'NAME')
    self.assertEquals(matcher.path, 'path[?(@.price<10)].val')
    self.assertFalse(matcher.FindReference())

  def testMatchInString(self):
    matcher = ReferenceMatcher('inside a $(ref.NAME.PATH) string')
    self.assertTrue(matcher.FindReference())
    self.assertEquals(matcher.name, 'NAME')
    self.assertEquals(matcher.path, 'PATH')
    self.assertFalse(matcher.FindReference())

  def testMatchTwo(self):
    matcher = ReferenceMatcher('two $(ref.NAME1.PATH1) inside '
                               'a $(ref.NAME2.PATH2) string')
    self.assertTrue(matcher.FindReference())
    self.assertEquals(matcher.name, 'NAME1')
    self.assertEquals(matcher.path, 'PATH1')
    self.assertTrue(matcher.FindReference())
    self.assertEquals(matcher.name, 'NAME2')
    self.assertEquals(matcher.path, 'PATH2')
    self.assertFalse(matcher.FindReference())

  def testMatchGoodAndBad(self):
    matcher = ReferenceMatcher('$(ref.NAME.PATH) good and $(ref.NAME.PATH bad')
    self.assertTrue(matcher.FindReference())
    self.assertEquals(matcher.name, 'NAME')
    self.assertEquals(matcher.path, 'PATH')
    try:
      matcher.FindReference()
      self.Fail('Expected Reference exception')
    except ExpansionReferenceError as e:
      self.assertTrue('Malformed' in e.message)
      self.assertTrue('$(ref.NAME.PATH bad' in e.message)

  def testAlmostMatch(self):
    matcher = ReferenceMatcher('inside a $(ref.NAME.PATH with no close paren')
    try:
      matcher.FindReference()
      self.Fail('Expected Reference exception')
    except ExpansionReferenceError as e:
      self.assertTrue('Malformed' in e.message)
      self.assertTrue('$(ref.NAME.PATH ' in e.message)

  # Tests for _TraverseNode

  def testFindAllReferences(self):
    ref_list = []
    node = {'a': ['a $(ref.name1.path1) string',
                  '$(ref.name2.path2)',
                  123,],
            'b': {'a1': 'another $(ref.name3.path3) string',},
            'c': 'yet another $(ref.name4.path4) string',}

    traversed_node = _TraverseNode(node, ref_list, None)

    self.assertEquals(node, traversed_node)
    self.assertEquals(4, len(ref_list))
    self.assertTrue(('name1', 'path1') in ref_list)
    self.assertTrue(('name2', 'path2') in ref_list)
    self.assertTrue(('name3', 'path3') in ref_list)
    self.assertTrue(('name4', 'path4') in ref_list)

  def testReplaceReference(self):
    ref_map = {'name1': {'path1a': '1a',
                         'path1b': '1b',},
               'name2': {'path2a': '2a',},}

    node = {'a': ['a $(ref.name1.path1a) string',
                  '$(ref.name2.path2a)',
                  123,],
            'b': {'a1': 'another $(ref.name1.path1b) string',},
            'c': 'yet another $(ref.name2.path2a)$(ref.name2.path2a) string',}

    expt = {'a': ['a 1a string',
                  '2a',
                  123,],
            'b': {'a1': 'another 1b string',},
            'c': 'yet another 2a2a string',}

    self.assertEquals(expt, _TraverseNode(node, None, ref_map))

  def testReplaceNotFoundReferencePath(self):
    ref_map = {'name1': {'path1a': '1a',},}

    node = {'a': ['a $(ref.name1.path1a) string',
                  'b $(ref.name1.path1b)',
                  'c $(ref.name2.path2a)'],}

    try:
      _TraverseNode(node, None, ref_map)
      self.Fail('Expected ExpansionReferenceError')
    except ExpansionReferenceError as e:
      self.assertTrue('No value found' in e.message)
      self.assertTrue('$(ref.name1.path1b)' in e.message)

  def testReplaceNotFoundReferenceName(self):
    ref_map = {'name1': {'path1a': '1a',},}

    node = {'a': ['a $(ref.name1.path1a) string',
                  'c $(ref.name2.path2a)'],}

    expt = {'a': ['a 1a string',
                  'c $(ref.name2.path2a)'],}

    self.assertEquals(expt, _TraverseNode(node, None, ref_map))

  # Tests for _ExtractWithJsonPath

  def testExtractFromList(self):
    ref_map = {'a': ['one', 'two', 'three',],}

    self.assertEquals('two', _ExtractWithJsonPath(ref_map, 'foo', 'a[1]'))

  def testExtractFromMap(self):
    ref_map = {'a': {'b': {'c': 'd'}}}

    self.assertEquals('d', _ExtractWithJsonPath(ref_map, 'foo', 'a.b.c'))

  def testExtractList(self):
    ref_map = {'a': ['one', 'two', 'three',],}

    self.assertEquals(['one', 'two', 'three',],
                      _ExtractWithJsonPath(ref_map, 'foo', 'a'))

  def testExtractListOfSingleItem(self):
    ref_map = {'a': ['one'],}

    self.assertEquals(['one'], _ExtractWithJsonPath(ref_map, 'foo', 'a'))

  def testExtractListWithWildcard(self):
    ref_map = {'a': ['one', 'two', 'three',],}

    self.assertEquals(['one', 'two', 'three',],
                      _ExtractWithJsonPath(ref_map, 'foo', 'a[*]'))

  def testExtractMap(self):
    ref_map = {'a': {'b': {'c': 'd'}}}

    self.assertEquals({'c': 'd'}, _ExtractWithJsonPath(ref_map, 'foo', 'a.b'))

  def testExtractFalse(self):
    ref_map = {'a': False}

    self.assertEquals(False, _ExtractWithJsonPath(ref_map, 'foo', 'a'))

  def testExtractFail_BadIndex(self):
    ref_map = {'a': ['one', 'two', 'three',],}

    # IndexError
    try:
      _ExtractWithJsonPath(ref_map, 'foo', 'a[3]')
      self.fail('Expected Reference error')
    except ExpansionReferenceError as e:
      self.assertTrue('foo.a[3]' in e.message)
      self.assertTrue('index out of range' in e.message)

    self.assertFalse(_ExtractWithJsonPath(ref_map, 'foo', 'a[3]',
                                          raise_exception=False))

  def testExtractFail_NotAList(self):
    ref_map = {'a': {'b': {'c': 'd'}}}

    try:
      _ExtractWithJsonPath(ref_map, 'foo', 'a.b[0]')
      self.fail('Expected Reference error')
    except ExpansionReferenceError as e:
      self.assertTrue('foo.a.b[0]' in e.message)
      self.assertTrue('No value found.' in e.message)

    self.assertFalse(_ExtractWithJsonPath(ref_map, 'foo', 'a.b[0]',
                                          raise_exception=False))

  def testExtractFail_BadKey(self):
    ref_map = {'a': {'b': {'c': 'd'}}}

    self.assertFalse(_ExtractWithJsonPath(ref_map, 'foo', 'a.b.d',
                                          raise_exception=False))

  def testExtractFail_NoObject(self):
    ref_map = {'a': {'b': {'c': 'd'}}}

    self.assertFalse(_ExtractWithJsonPath(ref_map, 'foo', 'a.b.c.d',
                                          raise_exception=False))

  def testExtractFail_MalformedPath(self):
    ref_map = {'a': {'b': {'c': 'd'}}}

    self.assertFalse(_ExtractWithJsonPath(ref_map, 'foo', 'a.b[2',
                                          raise_exception=False))


if __name__ == '__main__':
  unittest.main()
