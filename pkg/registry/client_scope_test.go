package registry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
)

type RegistryScopeTestSuite struct {
	TestSuite
}

func (suite *RegistryScopeTestSuite) SetupSuite() {
	// set registry use token auth
	dockerRegistry := setup(&suite.TestSuite, true, true, "token")
	// Start Docker registry
	go dockerRegistry.ListenAndServe()
}
func (suite *RegistryScopeTestSuite) TearDownSuite() {
	teardown(&suite.TestSuite)
	os.RemoveAll(suite.WorkspaceDir)
}

func (suite *RegistryScopeTestSuite) Test_1_Cehck_Push_Request_Scope() {

	//set simple gin auth server to check the push request scope
	r := gin.Default()
	r.GET("/auth", func(c *gin.Context) {
		suite.Equal(c.Request.URL.String(), string("/auth?scope=repository%3Atestrepo%2Flocal-subchart%3Apull%2Cpush&service=testservice"))
		c.JSON(http.StatusOK, gin.H{})
	})

	srv := &http.Server{
		Addr:    suite.AuthServerHost,
		Handler: r,
	}

	go srv.ListenAndServe()

	testingChartCreationTime := "1977-09-02T22:04:05Z"
	// basic push, good ref
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	_, err = suite.RegistryClient.Push(chartData, ref, PushOptCreationTime(testingChartCreationTime))
	suite.NotNil(err, "error pushing good ref because auth server don't give proper token")

	err = srv.Shutdown(context.Background())
	suite.Nil(err, "shut down simple gin server")
}

func (suite *RegistryScopeTestSuite) Test_2_Cehck_Pull_Request_Scope() {

	//set simple gin auth server to check the scope
	r := gin.Default()
	r.GET("/auth", func(c *gin.Context) {
		suite.Equal(c.Request.URL.String(), string("/auth?scope=repository%3Atestrepo%2Flocal-subchart%3Apull&service=testservice"))
		c.JSON(http.StatusOK, gin.H{})
	})
	srv := &http.Server{
		Addr:    suite.AuthServerHost,
		Handler: r,
	}
	go srv.ListenAndServe()

	// Load test chart (to build ref pushed in previous test)
	chartData, err := os.ReadFile("../downloader/testdata/local-subchart-0.1.0.tgz")
	suite.Nil(err, "no error loading test chart")
	meta, err := extractChartMeta(chartData)
	suite.Nil(err, "no error extracting chart meta")
	ref := fmt.Sprintf("%s/testrepo/%s:%s", suite.DockerRegistryHost, meta.Name, meta.Version)
	// Simple pull, chart only
	_, err = suite.RegistryClient.Pull(ref)
	suite.NotNil(err, "error pulling a simple chart because auth server don't give proper token")

	err = srv.Shutdown(context.Background())
	suite.Nil(err, "shut down simple gin server")
}

func TestRegistryScopeTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeTestSuite))
}
