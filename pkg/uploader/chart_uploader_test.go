package uploader

import (
	"fmt"
	"testing"

	"helm.sh/helm/v3/pkg/registry"
)

func TestUploadTo_InvalidFormatError(t *testing.T) {
	c := ChartUploader{}
	remote := "0.google.com:443/abc"
	got := c.UploadTo("", remote).Error()
	want := fmt.Errorf("invalid chart URL format: %s", remote).Error()

	if got != want {
		t.Errorf("expected error to be equal to %v, got %v", want, got)
	}
}

func TestUploadTo_SchemaPrefixMissingError(t *testing.T) {
	c := ChartUploader{}
	remote := "www.github.com/helm/helm"
	got := c.UploadTo("", remote).Error()
	want := fmt.Errorf("scheme prefix missing from remote (e.g. \"%s://\")", registry.OCIScheme).Error()

	if got != want {
		t.Errorf("expected error to be equal to %v, got %v", want, got)
	}
}
