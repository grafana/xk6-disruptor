// Package types defines the types supported by the API.
package disruptors

// HttpFault specifies a f to be injected in http requests
type HttpFault struct {
	// Average delay introduced to requests
	AverageDelay uint
	// Variation in the delay (with respect of the average delay)
	DelayVariation uint
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Error code to be returned by requests selected in the error rate
	ErrorCode uint
	// List of url paths to be excluded from disruptions
	Excluded []string
}
