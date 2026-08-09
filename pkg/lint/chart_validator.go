package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChartValidator provides utilities for validating Helm charts.
type ChartValidator struct {
_chartPath string
}

// NewChartValidator creates a new chart validator.
func NewChartValidator(chartPath string) *ChartValidator {
	return &ChartValidator{_chartPath: chartPath}
}

// ValidationResult contains the result of a validation check.
type ValidationResult struct {
	Passed  bool
	Message string
	Details string
}

// ValidateChart performs comprehensive validation of a Helm chart.
func (v *ChartValidator) ValidateChart() []ValidationResult {
	var results []ValidationResult

	results = append(results, v.ValidateChartYAML())
	results = append(results, v.ValidateTemplates())
	results = append(results, v.ValidateValues())
	results = append(results, v.ValidateDependencies())
	results = append(results, v.ValidateMetadata())

	return results
}

// ValidateChartYAML validates Chart.yaml structure.
func (v *ChartValidator) ValidateChartYAML() ValidationResult {
	chartYAML := filepath.Join(v._chartPath, "Chart.yaml")
	if _, err := os.Stat(chartYAML); os.IsNotExist(err) {
		return ValidationResult{
			Passed:  false,
			Message: "Chart.yaml not found",
			Details: "Every Helm chart must have a Chart.yaml file",
		}
	}

	content, err := os.ReadFile(chartYAML)
	if err != nil {
		return ValidationResult{
			Passed:  false,
			Message: "Failed to read Chart.yaml",
			Details: err.Error(),
		}
	}

	contentStr := string(content)
	requiredFields := []string{"apiVersion", "name", "version"}
	for _, field := range requiredFields {
		if !strings.Contains(contentStr, field+":") {
			return ValidationResult{
				Passed:  false,
				Message: fmt.Sprintf("Missing required field: %s", field),
				Details: fmt.Sprintf("Chart.yaml must contain '%s' field", field),
			}
		}
	}

	return ValidationResult{
		Passed:  true,
		Message: "Chart.yaml is valid",
	}
}

// ValidateTemplates validates templates directory.
func (v *ChartValidator) ValidateTemplates() ValidationResult {
	templatesDir := filepath.Join(v._chartPath, "templates")
	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return ValidationResult{
			Passed:  false,
			Message: "Templates directory not found",
			Details: "Charts should have a templates directory",
		}
	}

	var files []string
	filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		return ValidationResult{
			Passed:  false,
			Message: "No template files found",
			Details: "Templates directory is empty",
		}
	}

	return ValidationResult{
		Passed:  true,
		Message: fmt.Sprintf("Found %d template files", len(files)),
	}
}

// ValidateValues validates values.yaml.
func (v *ChartValidator) ValidateValues() ValidationResult {
	valuesYAML := filepath.Join(v._chartPath, "values.yaml")
	if _, err := os.Stat(valuesYAML); os.IsNotExist(err) {
		return ValidationResult{
			Passed:  false,
			Message: "values.yaml not found",
			Details: "Charts should have a values.yaml file",
		}
	}

	return ValidationResult{
		Passed:  true,
		Message: "values.yaml exists",
	}
}

// ValidateDependencies validates chart dependencies.
func (v *ChartValidator) ValidateDependencies() ValidationResult {
	chartYAML := filepath.Join(v._chartPath, "Chart.yaml")
	content, err := os.ReadFile(chartYAML)
	if err != nil {
		return ValidationResult{
			Passed:  true,
			Message: "Could not check dependencies",
		}
	}

	contentStr := string(content)
	if strings.Contains(contentStr, "dependencies:") {
		return ValidationResult{
			Passed:  true,
			Message: "Chart has dependencies defined",
		}
	}

	return ValidationResult{
		Passed:  true,
		Message: "No dependencies (optional)",
	}
}

// ValidateMetadata validates chart metadata.
func (v *ChartValidator) ValidateMetadata() ValidationResult {
	chartYAML := filepath.Join(v._chartPath, "Chart.yaml")
	content, err := os.ReadFile(chartYAML)
	if err != nil {
		return ValidationResult{
			Passed:  true,
			Message: "Could not check metadata",
		}
	}

	contentStr := string(content)
	optionalFields := []string{"description", "maintainers", "home", "sources"}
	found := 0
	for _, field := range optionalFields {
		if strings.Contains(contentStr, field+":") {
			found++
		}
	}

	return ValidationResult{
		Passed:  true,
		Message: fmt.Sprintf("Found %d/%d optional metadata fields", found, len(optionalFields)),
	}
}

// PrintReport prints a formatted validation report.
func PrintReport(results []ValidationResult) string {
	report := "Helm Chart Validation Report\n"
	report += "===========================\n\n"

	passed := 0
	failed := 0
	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
			failed++
		} else {
			passed++
		}
		report += fmt.Sprintf("[%s] %s\n", status, r.Message)
		if r.Details != "" {
			report += fmt.Sprintf("       %s\n", r.Details)
		}
	}

	report += fmt.Sprintf("\nResults: %d passed, %d failed\n", passed, failed)
	return report
}