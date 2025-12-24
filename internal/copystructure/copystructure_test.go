/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package copystructure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopy_Nil(t *testing.T) {
	result, err := Copy(nil)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{}, result)
}

func TestCopy_PrimitiveTypes(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{"bool", true},
		{"int", 42},
		{"int8", int8(8)},
		{"int16", int16(16)},
		{"int32", int32(32)},
		{"int64", int64(64)},
		{"uint", uint(42)},
		{"uint8", uint8(8)},
		{"uint16", uint16(16)},
		{"uint32", uint32(32)},
		{"uint64", uint64(64)},
		{"float32", float32(3.14)},
		{"float64", 3.14159},
		{"complex64", complex64(1 + 2i)},
		{"complex128", 1 + 2i},
		{"string", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Copy(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.input, result)
		})
	}
}

func TestCopy_Array(t *testing.T) {
	input := [3]int{1, 2, 3}
	result, err := Copy(input)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}

func TestCopy_Slice(t *testing.T) {
	t.Run("slice of ints", func(t *testing.T) {
		input := []int{1, 2, 3, 4, 5}
		result, err := Copy(input)
		require.NoError(t, err)

		resultSlice, ok := result.([]int)
		require.True(t, ok)
		assert.Equal(t, input, resultSlice)

		// Verify it's a deep copy by modifying original
		input[0] = 999
		assert.Equal(t, 1, resultSlice[0])
	})

	t.Run("slice of strings", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var input []int
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("slice of maps", func(t *testing.T) {
		input := []map[string]any{
			{"key1": "value1"},
			{"key2": "value2"},
		}
		result, err := Copy(input)
		require.NoError(t, err)

		resultSlice, ok := result.([]map[string]any)
		require.True(t, ok)
		assert.Equal(t, input, resultSlice)

		// Verify deep copy
		input[0]["key1"] = "modified"
		assert.Equal(t, "value1", resultSlice[0]["key1"])
	})
}

func TestCopy_Map(t *testing.T) {
	t.Run("map[string]any", func(t *testing.T) {
		input := map[string]any{
			"string": "value",
			"int":    42,
			"bool":   true,
			"nested": map[string]any{
				"inner": "value",
			},
		}

		result, err := Copy(input)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, input, resultMap)

		// Verify deep copy
		input["string"] = "modified"
		assert.Equal(t, "value", resultMap["string"])

		nestedInput := input["nested"].(map[string]any)
		nestedResult := resultMap["nested"].(map[string]any)
		nestedInput["inner"] = "modified"
		assert.Equal(t, "value", nestedResult["inner"])
	})

	t.Run("map[string]string", func(t *testing.T) {
		input := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result, err := Copy(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("nil map", func(t *testing.T) {
		var input map[string]any
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("map with nil values", func(t *testing.T) {
		input := map[string]any{
			"key1": "value1",
			"key2": nil,
		}

		result, err := Copy(input)
		require.NoError(t, err)

		resultMap, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, input, resultMap)
		assert.Nil(t, resultMap["key2"])
	})
}

func TestCopy_Struct(t *testing.T) {
	type TestStruct struct {
		Name     string
		Age      int
		Active   bool
		Scores   []int
		Metadata map[string]any
	}

	input := TestStruct{
		Name:   "John",
		Age:    30,
		Active: true,
		Scores: []int{95, 87, 92},
		Metadata: map[string]any{
			"level": "advanced",
			"tags":  []string{"go", "programming"},
		},
	}

	result, err := Copy(input)
	require.NoError(t, err)

	resultStruct, ok := result.(TestStruct)
	require.True(t, ok)
	assert.Equal(t, input, resultStruct)

	// Verify deep copy
	input.Name = "Modified"
	input.Scores[0] = 999
	assert.Equal(t, "John", resultStruct.Name)
	assert.Equal(t, 95, resultStruct.Scores[0])
}

func TestCopy_Pointer(t *testing.T) {
	t.Run("pointer to int", func(t *testing.T) {
		value := 42
		input := &value

		result, err := Copy(input)
		require.NoError(t, err)

		resultPtr, ok := result.(*int)
		require.True(t, ok)
		assert.Equal(t, *input, *resultPtr)

		// Verify they point to different memory locations
		assert.NotSame(t, input, resultPtr)

		// Verify deep copy
		*input = 999
		assert.Equal(t, 42, *resultPtr)
	})

	t.Run("pointer to struct", func(t *testing.T) {
		type Person struct {
			Name string
			Age  int
		}

		input := &Person{Name: "Alice", Age: 25}

		result, err := Copy(input)
		require.NoError(t, err)

		resultPtr, ok := result.(*Person)
		require.True(t, ok)
		assert.Equal(t, *input, *resultPtr)
		assert.NotSame(t, input, resultPtr)
	})

	t.Run("nil pointer", func(t *testing.T) {
		var input *int
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestCopy_Interface(t *testing.T) {
	t.Run("any with value", func(t *testing.T) {
		var input any = "hello"
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})

	t.Run("nil any", func(t *testing.T) {
		var input any
		result, err := Copy(input)
		require.NoError(t, err)
		// Copy(nil) returns an empty map according to the implementation
		assert.Equal(t, map[string]any{}, result)
	})

	t.Run("any with complex value", func(t *testing.T) {
		var input any = map[string]any{
			"key": "value",
			"nested": map[string]any{
				"inner": 42,
			},
		}

		result, err := Copy(input)
		require.NoError(t, err)
		assert.Equal(t, input, result)
	})
}

func TestCopy_ComplexNested(t *testing.T) {
	input := map[string]any{
		"users": []map[string]any{
			{
				"name": "Alice",
				"age":  30,
				"addresses": []map[string]any{
					{"type": "home", "city": "NYC"},
					{"type": "work", "city": "SF"},
				},
			},
			{
				"name": "Bob",
				"age":  25,
				"addresses": []map[string]any{
					{"type": "home", "city": "LA"},
				},
			},
		},
		"metadata": map[string]any{
			"version": "1.0",
			"flags":   []bool{true, false, true},
		},
	}

	result, err := Copy(input)
	require.NoError(t, err)

	resultMap, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, input, resultMap)

	// Verify deep copy by modifying nested values
	users := input["users"].([]map[string]any)
	addresses := users[0]["addresses"].([]map[string]any)
	addresses[0]["city"] = "Modified"

	resultUsers := resultMap["users"].([]map[string]any)
	resultAddresses := resultUsers[0]["addresses"].([]map[string]any)
	assert.Equal(t, "NYC", resultAddresses[0]["city"])
}

func TestCopy_Functions(t *testing.T) {
	t.Run("function", func(t *testing.T) {
		input := func() string { return "hello" }
		result, err := Copy(input)
		require.NoError(t, err)

		// Functions should be copied as-is (same reference)
		resultFunc, ok := result.(func() string)
		require.True(t, ok)
		assert.Equal(t, input(), resultFunc())
	})

	t.Run("nil function", func(t *testing.T) {
		var input func()
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestCopy_Channels(t *testing.T) {
	t.Run("channel", func(t *testing.T) {
		input := make(chan int, 1)
		input <- 42

		result, err := Copy(input)
		require.NoError(t, err)

		// Channels should be copied as-is (same reference)
		resultChan, ok := result.(chan int)
		require.True(t, ok)

		// Since channels are copied as references, verify we can read from the result channel
		value := <-resultChan
		assert.Equal(t, 42, value)
	})

	t.Run("nil channel", func(t *testing.T) {
		var input chan int
		result, err := Copy(input)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}
