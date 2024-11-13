package collector

import "context"

type Collector interface {
	Run(context.Context) error
	Stop()
}
