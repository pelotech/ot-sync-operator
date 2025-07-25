package contollerutils

import "time"

type OperatorConfig struct {
	Concurrency          int
	RetryLimit           int
	RetryBackoffDuration time.Duration
}
