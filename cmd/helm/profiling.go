// Profile CPU and memory usage of Helm commands

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/spf13/cobra"
)

var (
	cpuProfileFile *os.File
)

// startProfiling starts profiling CPU usage
func startProfiling(cpuprofile string) error {
	if cpuprofile != "" {
		var err error
		cpuProfileFile, err = os.Create(cpuprofile)
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

// stopProfiling stops profiling CPU and memory usage and writes the results to
// the files specified by --cpuprofile and --memprofile flags respectively.
func stopProfiling(memprofile string) error {
	errs := []string{}

	// Stop CPU profiling if it was started
	if cpuProfileFile != nil {
		pprof.StopCPUProfile()
		err := cpuProfileFile.Close()
		if err != nil {
			errs = append(errs, err.Error())
		}
		cpuProfileFile = nil
	}

	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			errs = append(errs, err.Error())
		}
		defer f.Close()

		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors while stopping profiling: [%s]", strings.Join(errs, ", "))
	}

	return nil
}

// addProfilingFlags adds the --cpuprofile and --memprofile flags to the given command.
func addProfilingFlags(cmd *cobra.Command) {
	// Persistent flags to make available to subcommands
	cmd.PersistentFlags().String("cpuprofile", "", "File path to write cpu profiling data")
	cmd.PersistentFlags().String("memprofile", "", "File path to write memory profiling data")
}
