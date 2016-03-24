package main

import (
	"net/http"
	"testing"
)

func TestListChartRepositories(t *testing.T) {
	c := stubContext()
	s := httpHarness(c, "GET /repositories", listChartReposHandlerFunc)
	defer s.Close()

	res, err := http.Get(s.URL + "/repositories")
	if err != nil {
		t.Errorf("Failed GET: %s", err)
	} else if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}
}
