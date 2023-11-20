package stressors

import (
	"context"
	"crypto/sha1" //nolint:gosec
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

// DefaultSlice default CPU stress slice
const DefaultSlice = 100 * time.Millisecond

// CPUStressor defines a stressor for CPU
type CPUStressor struct {
	Slice time.Duration
	Load  int
}

// CPUDisruption defines a disruption that stress the CPU
type CPUDisruption struct {
	Load int
	CPUs int
}

// Apply stresses one CPU until the context is done
// This code is based on the cpu stress routing in stress-ng
func (s *CPUStressor) Apply(ctx context.Context) error {
	// scheduleTime is used to compensate time go routine is not scheduled
	scheduleTime := 0.0
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// lock the goroutine to a thread to ensure consistent CPU reads
			runtime.LockOSThread()

			clockStart := time.Now()
			cpuStart := CPUTime()

			buff := make([]byte, 1000)

			// consume a slice of CPU
			for CPUTime().Nanoseconds() < cpuStart.Nanoseconds()+s.Slice.Nanoseconds() {
				_ = sha1.Sum(buff) //nolint:gosec
			}

			// calculate how much CPU time was actually consumed in the busy cycle
			busy := CPUTime() - cpuStart

			// calculate how long it took to consume the CPU slice
			elapsed := time.Since(clockStart)

			runtime.UnlockOSThread()

			// calculate the time that must sleep to get the target percentage of CPU consumption.
			// If  B = CPU time consumed, I = idle time and L = Load, then
			// L = 100*B/(B+I) --> I = B*(100-L)/L
			//
			// The following formula uses this relationship and adjusts for any idle time consuming CPU (busy-elapsed)
			// and idle time not accounted from previous cycle (scheduleTime)
			idle := float64(int64(100-s.Load)*int64(busy))/float64(s.Load) + float64(busy-elapsed) - scheduleTime

			if idle < 0.0 {
				scheduleTime = 0.0
				continue
			}

			startSleep := time.Now()
			time.Sleep(time.Duration(idle))

			// scheduleTime compensates for the time it takes to re-schedule the goroutine after sleep
			scheduleTime = float64(time.Since(startSleep)) - idle
		}
	}
}

// VMStressor defines a stressor for memroy
type VMStressor struct {
	VMBytes uint64
}

// MemoryDisruption defines a disruption that stress the memeory
type MemoryDisruption struct {
	VMSize uint64
}

// Apply stresses Virtual memory
func (s *VMStressor) Apply(ctx context.Context) error {
	if s.VMBytes == 0 {
		return nil
	}
	mmapFlags := syscall.MAP_PRIVATE | syscall.MAP_ANONYMOUS
	mmapProt := syscall.PROT_READ | syscall.PROT_WRITE
	mmapSize := int(s.VMBytes & ^(uint64(os.Getpagesize()) - 1)) // round up to pageSize
	buff, err := syscall.Mmap(0, 0, mmapSize, mmapProt, mmapFlags)
	if err != nil {
		return fmt.Errorf("mapping virtual memory: %w", err)
	}
	defer func() {
		_ = syscall.Munmap(buff)
	}()

	// wait context end
	<-ctx.Done()
	return nil
}

// ResourceDisruption defines a disruption that stress the CPU and Memory of a target
type ResourceDisruption struct {
	CPUDisruption
	MemoryDisruption
}

// ResourceStressOptions defines options that control the resource stressing
type ResourceStressOptions struct {
	// Slice defines the interval of CPU stress. Default 100ms
	// Each slice is divided between busy and idle times to achieve a target load
	// Smaller slices should have smoother cpu consumption
	Slice time.Duration
}

// ResourceStressor defines a resource stressor
type ResourceStressor struct {
	Options    ResourceStressOptions
	Disruption ResourceDisruption
}

// NewResourceStressor creates a new ResourceStressor using the given options
func NewResourceStressor(disruption ResourceDisruption, options ResourceStressOptions) (*ResourceStressor, error) {
	if options.Slice == 0 {
		options.Slice = DefaultSlice
	}

	return &ResourceStressor{
		Options:    options,
		Disruption: disruption,
	}, nil
}

// Apply applies the resource stress disruption for a given duration
func (r *ResourceStressor) Apply(ctx context.Context, duration time.Duration) error {
	// one stressor goroutine per CPU and one for memory
	stressRoutines := r.Disruption.CPUs + 1

	stressorsCtx, done := context.WithTimeout(ctx, duration)
	defer done()

	doneCh := make(chan error, stressRoutines)

	// create a CPUStressor for each CPU
	for i := 0; i < r.Disruption.CPUs; i++ {
		go func() {
			s := CPUStressor{
				Slice: r.Options.Slice,
				Load:  r.Disruption.Load,
			}
			doneCh <- s.Apply(stressorsCtx)
		}()
	}

	// create a goroutine for memory stress
	go func() {
		m := VMStressor{
			VMBytes: r.Disruption.VMSize,
		}
		doneCh <- m.Apply(stressorsCtx)
	}()

	// wait for all stressors to finish or context to be done
	pending := stressRoutines
	for pending > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-doneCh:
			pending--
			if err != nil {
				return err
			}
		}
	}

	return nil
}
