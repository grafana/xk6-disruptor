package runtime

import (
	"fmt"
	"os"
	goruntime "runtime"
	"runtime/pprof"
	"runtime/trace"
)

// ProfilerConfig defines the configuration of a Profiler
type ProfilerConfig struct {
	CPUProfile         bool
	CPUProfileFileName string
	MemProfile         bool
	MemProfileRate     int
	MemProfileFileName string
	Trace              bool
	TraceFileName      string
}

// Profiler defines the methods to control execution profiling
type Profiler interface {
	// Start initiates the tracing. If already active, has no effect
	Start() error
	// Stop terminates the tracing. If not started, has no effect
	Stop() error
}

// profiler maintains the configuration state of the profiler
type profiler struct {
	ProfilerConfig
	active         bool
	cpuProfileFile *os.File
	memProfileFile *os.File
	traceFile      *os.File
}

// NewProfiler creates a Profiler instance
func NewProfiler(config ProfilerConfig) (Profiler, error) {
	if config.MemProfile {
		if config.MemProfileRate < 0 {
			return nil, fmt.Errorf("memory rate must be non-negative: %d", config.MemProfileRate)
		}

		if config.MemProfileFileName == "" {
			return nil, fmt.Errorf("memory profile file name cannot be empty")
		}
	}

	if config.CPUProfile {
		if config.CPUProfileFileName == "" {
			return nil, fmt.Errorf("CPU profile file name cannot be empty")
		}
	}

	if config.Trace {
		if config.TraceFileName == "" {
			return nil, fmt.Errorf("trace output file name cannot be empty")
		}
	}

	return &profiler{
		ProfilerConfig: config,
		active:         false,
	}, nil
}

func (p *profiler) Start() error {
	if p.active {
		return nil
	}

	var err error
	// cpu profiling
	if p.CPUProfile {
		p.cpuProfileFile, err = os.Create(p.CPUProfileFileName)
		if err != nil {
			return fmt.Errorf("error creating CPU profiling file %q: %w", p.CPUProfileFileName, err)
		}

		err = pprof.StartCPUProfile(p.cpuProfileFile)
		if err != nil {
			return fmt.Errorf("failed to start CPU profiling: %w", err)
		}
	}

	// memory profiling
	if p.MemProfile {
		p.memProfileFile, err = os.Create(p.MemProfileFileName)
		if err != nil {
			return fmt.Errorf("error creating memory profiling file %q: %w", p.MemProfileFileName, err)
		}

		goruntime.MemProfileRate = p.MemProfileRate
	}

	// trace program execution
	if p.Trace {
		p.traceFile, err = os.Create(p.TraceFileName)
		if err != nil {
			return fmt.Errorf("failed to create trace output file %q: %w", p.TraceFileName, err)
		}

		if err := trace.Start(p.traceFile); err != nil {
			return fmt.Errorf("failed to start trace: %w", err)
		}
	}

	return nil
}

func (p *profiler) Stop() error {
	if !p.active {
		return nil
	}

	if p.CPUProfile {
		pprof.StopCPUProfile()
	}

	if p.MemProfile {
		err := pprof.Lookup("heap").WriteTo(p.memProfileFile, 0)
		if err != nil {
			return fmt.Errorf("failed to write memory profile to file %q: %w", p.MemProfileFileName, err)
		}
	}

	if p.Trace {
		trace.Stop()
		_ = p.traceFile.Close()
	}

	return nil
}
