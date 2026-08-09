package lint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateChartYAML_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewChartValidator(tmpDir)
	result := validator.ValidateChartYAML()

	if result.Passed {
		t.Error("Expected validation to fail for missing Chart.yaml")
	}
}

func TestValidateChartYAML_Present(t *testing.T) {
	tmpDir := t.TempDir()
	chartYAML := piVersion: v2
name: test-chart
version: 0.1.0
description: A test chart

	os.WriteFile(filepath.Join(tmpDir, "Chart.yaml"), []byte(chartYAML), 0644)

	validator := NewChartValidator(tmpDir)
	result := validator.ValidateChartYAML()

	if !result.Passed {
		t.Errorf("Expected validation to pass, got: %s", result.Message)
	}
}

func TestValidateTemplates_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewChartValidator(tmpDir)
	result := validator.ValidateTemplates()

	if result.Passed {
		t.Error("Expected validation to fail for missing templates directory")
	}
}

func TestValidateTemplates_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0755)

	validator := NewChartValidator(tmpDir)
	result := validator.ValidateTemplates()

	if result.Passed {
		t.Error("Expected validation to fail for empty templates directory")
	}
}

func TestValidateTemplates_HasFiles(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)
	os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte("apiVersion: apps/v1"), 0644)

	validator := NewChartValidator(tmpDir)
	result := validator.ValidateTemplates()

	if !result.Passed {
		t.Errorf("Expected validation to pass, got: %s", result.Message)
	}
}

func TestValidateValues_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewChartValidator(tmpDir)
	result := validator.ValidateValues()

	if result.Passed {
		t.Error("Expected validation to fail for missing values.yaml")
	}
}

func TestValidateValues_Present(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("replicaCount: 1"), 0644)

	validator := NewChartValidator(tmpDir)
	result := validator.ValidateValues()

	if !result.Passed {
		t.Errorf("Expected validation to pass, got: %s", result.Message)
	}
}

func TestValidateChart_Full(t *testing.T) {
	tmpDir := t.TempDir()

	chartYAML := piVersion: v2
name: test-chart
version: 0.1.0
description: A test chart

	os.WriteFile(filepath.Join(tmpDir, "Chart.yaml"), []byte(chartYAML), 0644)
	os.WriteFile(filepath.Join(tmpDir, "values.yaml"), []byte("replicaCount: 1"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "deployment.yaml"), []byte("apiVersion: apps/v1"), 0644)

	validator := NewChartValidator(tmpDir)
	results := validator.ValidateChart()

	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}

	if passed < 4 {
		t.Errorf("Expected at least 4 passed validations, got %d", passed)
	}
}

func TestPrintReport(t *testing.T) {
	results := []ValidationResult{
		{Passed: true, Message: "Test passed"},
		{Passed: false, Message: "Test failed", Details: "Something went wrong"},
	}

	report := PrintReport(results)
	if report == "" {
		t.Error("Expected non-empty report")
	}
}