# Appendix: Go Data Types and Templates

The Helm template language is implemented in the strongly typed Go programming language. For that reason, variables in templates are _typed_. For the most part, variables will be exposed as one of the following types:

- string: A string of text
- bool: a `true` or `false`
- int: An integer value (there are also 8, 16, 32, and 64 bit signed and unsigned variants of this)
- float64: a 64-bit floating point value (there are also 8, 16, and 32 bit varieties of this)
- a byte slice (`[]byte`), often used to hold (potentially) binary data
- struct: an object with properties and methods
- a slice (indexed list) of one of the previous types
- a string-keyed map (`map[string]interface{}`) where the value is one of the previous types

There are many other types in Go, and sometimes you will have to convert between them in your templates. The easiest way to debug an object's type is to pass it through `printf "%t"` in a template, which will print the type. Also see the `typeOf` and `kindOf` functions.