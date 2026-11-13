package main

import (
	"context"

	"dagger/engine-dev/internal/dagger"

	daggerio "dagger.io/dagger"
	"github.com/dagger/querybuilder"
)

// withDiskTemp is ctr with an empty directory at path on the engine's disk,
// a new one for each process that runs with it, removed when that process
// ends: withMountedTemp(disk: true). A nested engine's state goes in one, so
// each engine starts on state of its own, and nothing named for the run keys
// the containers that use the engine.
//
// The published SDK this module builds against has no field for disk yet,
// so the call is selected by hand. This goes when the SDK has it.
func withDiskTemp(ctx context.Context, ctr *dagger.Container, path string) (*dagger.Container, error) {
	id, err := ctr.ID(ctx)
	if err != nil {
		return nil, err
	}
	q := querybuilder.Query().Client(daggerio.DefaultGraphQLClient()).
		Select("node").Arg("id", id).InlineFragment("Container").
		Select("withMountedTemp").Arg("path", path).Arg("disk", true)
	return (&dagger.Container{}).WithGraphQLQuery(q), nil
}
