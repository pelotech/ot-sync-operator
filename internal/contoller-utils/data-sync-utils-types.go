package contollerutils

import "time"

type HelmChartInformation struct {
	// Helm Chart Configuration
	RepoURL      string
	ChartName    string
	ChartVersion string
}

type BehaviorConfig struct {
	Concurrency          int
	RetryLimit           int
	RetryBackoffDuration time.Duration
}

// OperatorConfig holds the configuration for the operator and the Helm chart it deploys.
type OperatorConfig struct {
	HelmChartInfo  HelmChartInformation
	BehaviorConfig BehaviorConfig
}
