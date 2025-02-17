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

package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

var (
	cpuProfileFile *os.File
	cpuProfilePath string
	memProfilePath string
)

func init() {
	cpuProfilePath = os.Getenv("HELM_PPROF_CPU_PROFILE")
	memProfilePath = os.Getenv("HELM_PPROF_MEM_PROFILE")
}

// startProfiling starts profiling CPU usage if HELM_PPROF_CPU_PROFILE is set
// to a file path. It returns an error if the file could not be created or
// CPU profiling could not be started.
func startProfiling() error {
	if cpuProfilePath != "" {
		var err error
		cpuProfileFile, err = os.Create(cpuProfilePath)
		if err != nil {
			return fmt.Errorf("could not create CPU profile: %w", err)
		}
		if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
			cpuProfileFile.Close()
			cpuProfileFile = nil
			return fmt.Errorf("could not start CPU profile: %w", err)
		}
	}
	return nil
}

// stopProfiling stops profiling CPU and memory usage.
// It writes memory profile to the file path specified in HELM_PPROF_MEM_PROFILE
// environment variable.
func stopProfiling() error {
	errs := []error{}

	// Stop CPU profiling if it was started
	if cpuProfileFile != nil {
		pprof.StopCPUProfile()
		err := cpuProfileFile.Close()
		if err != nil {
			errs = append(errs, err)
		}
		cpuProfileFile = nil
	}

	if memProfilePath != "" {
		f, err := os.Create(memProfilePath)
		if err != nil {
			errs = append(errs, err)
		}
		defer f.Close()

		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			errs = append(errs, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("error(s) while stopping profiling: %w", err)
	}

	return nil
}
