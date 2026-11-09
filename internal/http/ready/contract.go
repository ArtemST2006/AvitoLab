// Package ready serves the readiness endpoint.
package ready

import "context"

//go:generate go tool mockgen -source=contract.go -destination=mocks_test.go -package=ready_test
type database interface {
	Ping(context.Context) error
}
