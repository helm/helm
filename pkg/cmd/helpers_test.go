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

package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	shellwords "github.com/mattn/go-shellwords"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/internal/test"
	"helm.sh/helm/v4/pkg/action"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/cli"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
	"helm.sh/helm/v4/pkg/time"
)

func testTimestamper() time.Time { return time.Unix(242085845, 0).UTC() }

func init() {
	action.Timestamper = testTimestamper
}

func runTestCmd(t *testing.T, tests []cmdTestCase) {
	t.Helper()
	for _, tt := range tests {
		for i := 0; i <= tt.repeat; i++ {
			t.Run(tt.name, func(t *testing.T) {
				defer resetEnv()()

				storage := storageFixture()
				for _, rel := range tt.rels {
					if err := storage.Create(rel); err != nil {
						t.Fatal(err)
					}
				}
				t.Logf("running cmd (attempt %d): %s", i+1, tt.cmd)
				_, out, err := executeActionCommandC(storage, tt.cmd)
				if tt.wantError && err == nil {
					t.Errorf("expected error, got success with the following output:\n%s", out)
				}
				if !tt.wantError && err != nil {
					t.Errorf("expected no error, got: '%v'", err)
				}
				if tt.golden != "" {
					test.AssertGoldenString(t, out, tt.golden)
				}
			})
		}
	}
}

func storageFixture() *storage.Storage {
	return storage.Init(driver.NewMemory())
}

func executeActionCommandC(store *storage.Storage, cmd string) (*cobra.Command, string, error) {
	return executeActionCommandStdinC(store, nil, cmd)
}

func executeActionCommandStdinC(store *storage.Storage, in *os.File, cmd string) (*cobra.Command, string, error) {
	args, err := shellwords.Parse(cmd)
	if err != nil {
		return nil, "", err
	}

	buf := new(bytes.Buffer)

	actionConfig := &action.Configuration{
		Releases:     store,
		KubeClient:   &kubefake.PrintingKubeClient{Out: io.Discard},
		Capabilities: chartutil.DefaultCapabilities,
	}

	root, err := newRootCmdWithConfig(actionConfig, buf, args, SetupLogging)
	if err != nil {
		return nil, "", err
	}

	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	oldStdin := os.Stdin
	if in != nil {
		root.SetIn(in)
		os.Stdin = in
	}

	if mem, ok := store.Driver.(*driver.Memory); ok {
		mem.SetNamespace(settings.Namespace())
	}
	c, err := root.ExecuteC()

	result := buf.String()

	os.Stdin = oldStdin

	return c, result, err
}

// cmdTestCase describes a test case that works with releases.
type cmdTestCase struct {
	name      string
	cmd       string
	golden    string
	wantError bool
	// Rels are the available releases at the start of the test.
	rels []*release.Release
	// Number of repeats (in case a feature was previously flaky and the test checks
	// it's now stably producing identical results). 0 means test is run exactly once.
	repeat int
}

func executeActionCommand(cmd string) (*cobra.Command, string, error) {
	return executeActionCommandC(storageFixture(), cmd)
}

func resetEnv() func() {
	origEnv := os.Environ()
	return func() {
		os.Clearenv()
		for _, pair := range origEnv {
			kv := strings.SplitN(pair, "=", 2)
			os.Setenv(kv[0], kv[1])
		}
		settings = cli.New()
	}
}

func TestCmdGetDryRunFlagStrategy(t *testing.T) {

	type testCaseExpectedLog struct {
		Level string
		Msg   string
	}
	testCases := map[string]struct {
		DryRunFlagArg    string
		IsTemplate       bool
		ExpectedStrategy action.DryRunStrategy
		ExpectedError    bool
		ExpectedLog      *testCaseExpectedLog
	}{
		"unset_value": {
			DryRunFlagArg:    "--dry-run",
			ExpectedStrategy: action.DryRunClient,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `--dry-run is deprecated and should be replaced with '--dry-run=client'`,
			},
		},
		"unset_special": {
			DryRunFlagArg:    "--dry-run=unset", // Special value that matches cmd.Flags("dry-run").NoOptDefVal
			ExpectedStrategy: action.DryRunClient,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `--dry-run is deprecated and should be replaced with '--dry-run=client'`,
			},
		},
		"none": {
			DryRunFlagArg:    "--dry-run=none",
			ExpectedStrategy: action.DryRunNone,
		},
		"client": {
			DryRunFlagArg:    "--dry-run=client",
			ExpectedStrategy: action.DryRunClient,
		},
		"server": {
			DryRunFlagArg:    "--dry-run=server",
			ExpectedStrategy: action.DryRunServer,
		},
		"bool_false": {
			DryRunFlagArg:    "--dry-run=false",
			ExpectedStrategy: action.DryRunNone,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `boolean '--dry-run=false' flag is deprecated and must be replaced with '--dry-run=none'`,
			},
		},
		"bool_true": {
			DryRunFlagArg:    "--dry-run=true",
			ExpectedStrategy: action.DryRunClient,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `boolean '--dry-run=true' flag is deprecated and must be replaced with '--dry-run=client'`,
			},
		},
		"bool_0": {
			DryRunFlagArg:    "--dry-run=0",
			ExpectedStrategy: action.DryRunNone,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `boolean '--dry-run=0' flag is deprecated and must be replaced with '--dry-run=none'`,
			},
		},
		"bool_1": {
			DryRunFlagArg:    "--dry-run=1",
			ExpectedStrategy: action.DryRunClient,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `boolean '--dry-run=1' flag is deprecated and must be replaced with '--dry-run=client'`,
			},
		},
		"invalid": {
			DryRunFlagArg: "--dry-run=invalid",
			ExpectedError: true,
		},
		"template_unset_value": {
			DryRunFlagArg:    "--dry-run",
			IsTemplate:       true,
			ExpectedStrategy: action.DryRunClient,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `--dry-run is deprecated and should be replaced with '--dry-run=client'`,
			},
		},
		"template_bool_false": {
			DryRunFlagArg: "--dry-run=false",
			IsTemplate:    true,
			ExpectedError: true,
		},
		"template_bool_template_true": {
			DryRunFlagArg:    "--dry-run=true",
			IsTemplate:       true,
			ExpectedStrategy: action.DryRunClient,
			ExpectedLog: &testCaseExpectedLog{
				Level: "WARN",
				Msg:   `boolean '--dry-run=true' flag is deprecated and must be replaced with '--dry-run=client'`,
			},
		},
		"template_none": {
			DryRunFlagArg: "--dry-run=none",
			IsTemplate:    true,
			ExpectedError: true,
		},
		"template_client": {
			DryRunFlagArg:    "--dry-run=client",
			IsTemplate:       true,
			ExpectedStrategy: action.DryRunClient,
		},
		"template_server": {
			DryRunFlagArg:    "--dry-run=server",
			IsTemplate:       true,
			ExpectedStrategy: action.DryRunServer,
		},
	}

	for name, tc := range testCases {

		logBuf := new(bytes.Buffer)
		logger := slog.New(slog.NewJSONHandler(logBuf, nil))
		slog.SetDefault(logger)

		cmd := &cobra.Command{
			Use: "helm",
		}
		addDryRunFlag(cmd)
		cmd.Flags().Parse([]string{"helm", tc.DryRunFlagArg})

		t.Run(name, func(t *testing.T) {
			dryRunStrategy, err := cmdGetDryRunFlagStrategy(cmd, tc.IsTemplate)
			if tc.ExpectedError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.ExpectedStrategy, dryRunStrategy)
			}

			if tc.ExpectedLog != nil {
				logResult := map[string]string{}
				err = json.Unmarshal(logBuf.Bytes(), &logResult)
				require.Nil(t, err)

				assert.Equal(t, tc.ExpectedLog.Level, logResult["level"])
				assert.Equal(t, tc.ExpectedLog.Msg, logResult["msg"])
			} else {
				assert.Equal(t, 0, logBuf.Len())
			}
		})
	}
}
