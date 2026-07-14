package main

// Regenerate the testify mocks declared in .mockery.yaml. This uses the pinned
// mockery tool dependency recorded in go.mod (added via `go get -tool`), so it
// runs the same version everywhere without a separate install.
//
//go:generate go tool mockery
