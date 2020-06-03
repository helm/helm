package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIntoString(t *testing.T) {
	need := require.New(t)
	is := assert.New(t)

	dest := make(map[string]string)
	goodFlag := "foo.txt=../foo.txt"
	anotherFlag := " bar.txt=~/bar.txt, baz.txt=/path/to/baz.txt"

	err := ParseIntoString(goodFlag, dest)
	need.NoError(err)

	err = ParseIntoString(anotherFlag, dest)
	need.NoError(err)

	is.Contains(dest, "foo.txt")
	is.Contains(dest, "bar.txt")
	is.Contains(dest, "baz.txt")

	is.Equal(dest["foo.txt"], "../foo.txt", "foo.txt not mapped properly")
	is.Equal(dest["bar.txt"], "~/bar.txt", "bar.txt not mapped properly")
	is.Equal(dest["baz.txt"], "/path/to/baz.txt", "baz.txt not mapped properly")

	overwriteFlag := "foo.txt=../new_foo.txt"
	err = ParseIntoString(overwriteFlag, dest)
	need.NoError(err)

	is.Equal(dest["foo.txt"], "../new_foo.txt")

	badFlag := "empty.txt"
	err = ParseIntoString(badFlag, dest)
	is.NotNil(err)
}
