package kubectl

import (
	"testing"
)

func TestPrintCreate(t *testing.T) {
	var client Runner = PrintRunner{}

	expected := `[CMD] kubectl create -f - < some stdin data`

	out, err := client.Create([]byte("some stdin data"))
	if err != nil {
		t.Error(err)
	}

	actual := string(out)

	if expected != actual {
		t.Fatalf("actual %s != expected %s", actual, expected)
	}
}
