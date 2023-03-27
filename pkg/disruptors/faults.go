package disruptors

// HTTPFault specifies a fault to be injected in http requests
type HTTPFault struct {
	// port the disruptions will be applied to
	Port uint
	// Average delay introduced to requests
	AverageDelay uint `js:"averageDelay"`
	// Variation in the delay (with respect of the average delay)
	DelayVariation uint `js:"delayVariation"`
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32 `js:"errorRate"`
	// Error code to be returned by requests selected in the error rate
	ErrorCode uint `js:"errorCode"`
	// Body to be returned when an error is injected
	ErrorBody string `js:"errorBody"`
	// Comma-separated list of url paths to be excluded from disruptions
	Exclude string
}
