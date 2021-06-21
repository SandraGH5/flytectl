package execution

import (
	"github.com/flyteorg/flytectl/pkg/filters"
)

//go:generate pflags Config --default-var DefaultConfig --bind-default-var
var (
	DefaultConfig = &Config{
		Filter: filters.DefaultFilter,
	}
)

// Config
type Config struct {
	Filter filters.Filters `json:"filter" pflag:","`
	Details bool `json:"details" pflag:",gets node execution details."`
}
