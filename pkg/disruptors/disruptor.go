package disruptors

// Disruptor defines the generic interface implemented by all disruptors
type Disruptor interface {
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
}
