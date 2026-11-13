package main

import (
	"dagger/engine-dev/internal/dagger"
	"dagger/engine-dev/internal/dagger/clients/disktemp"
)

// withDiskTemp is ctr with an empty directory at path on the engine's disk,
// a new one for each process that runs with it, removed when that process
// ends: withMountedTemp(disk: true). A nested engine's state goes in one, so
// each engine starts on state of its own, and nothing named for the run keys
// the containers that use the engine.
//
// This module's API view has no disk argument, so the mount is the
// disktemp module's, on a view that has it. This goes when a release of
// the SDK has disk.
func withDiskTemp(ctr *dagger.Container, path string) *dagger.Container {
	return disktemp.New().Mount(ctr, path)
}
