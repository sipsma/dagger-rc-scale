// Wolfi containers for dagger/dagger's own modules, each installing a named
// set of packages pinned to exact versions.
//
// The Wolfi module resolves packages, with all their dependencies, against
// Wolfi's index as it is when the call runs, and that index isn't part of any
// cache key. Wolfi publishes new builds most days, so two engines that build
// the same container at different times would install different packages,
// and everything built on top of it would differ too. Installing a locked set
// makes a container change only when wolfi.lock.json does.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"runtime"
	"slices"

	"dagger.io/dagger/core"
	"main/internal/dagger"
	"main/internal/dagger/clients/alpine"
	"main/internal/dagger/clients/wolfi"
)

// The resolved packages of every set, written by Refresh
//
//go:embed wolfi.lock.json
var lockJSON []byte

type lockFile struct {
	Sets map[string]lockedSet `json:"sets"`
}

type lockedSet struct {
	// The set's packages when it was resolved
	Packages []string `json:"packages"`
	// The resolved packages as "name=version", by GOARCH
	Locked map[string][]string `json:"locked"`
}

// A container's packages, before their dependencies are resolved
type packageSet struct {
	packages []string
	// The architectures to resolve the set for, as GOARCH (default: every
	// architecture Wolfi publishes)
	arches []string
}

// The architectures Wolfi publishes packages for, as GOARCH
var arches = []string{"amd64", "arm64"}

func readLock() (lockFile, error) {
	var lock lockFile
	if err := json.Unmarshal(lockJSON, &lock); err != nil {
		return lock, fmt.Errorf("parse wolfi.lock.json: %w", err)
	}
	return lock, nil
}

type WolfiLock struct{}

// A Wolfi container with the named set's locked packages installed
func (m *WolfiLock) Container(
	// The package set's name, from sets.go
	set string,
	// Hardware architecture to target, as GOARCH (default: the engine's)
	// +optional
	arch string,
) (*dagger.Container, error) {
	if arch == "" {
		arch = runtime.GOARCH
	}
	want, ok := sets[set]
	if !ok {
		return nil, fmt.Errorf("no package set %q", set)
	}
	lock, err := readLock()
	if err != nil {
		return nil, err
	}
	s, ok := lock.Sets[set]
	if !ok || !slices.Equal(s.Packages, want.packages) {
		return nil, fmt.Errorf("wolfi.lock.json doesn't match package set %q; run refresh", set)
	}
	pkgs, ok := s.Locked[arch]
	if !ok {
		return nil, fmt.Errorf("wolfi.lock.json has no %s packages for set %q; run refresh", arch, set)
	}
	return wolfi.New().Container(wolfi.WolfiContainerOpts{Packages: pkgs, Arch: arch}), nil
}

// Resolve every set's packages to the newest versions Wolfi offers now, and
// return the new wolfi.lock.json for this module's directory. Run it after
// changing a set in sets.go.
// +cache="session"
func (m *WolfiLock) Refresh(ctx context.Context) (*dagger.File, error) {
	lock := lockFile{Sets: map[string]lockedSet{}}
	for _, name := range slices.Sorted(maps.Keys(sets)) {
		set := sets[name]
		setArches := set.arches
		if len(setArches) == 0 {
			setArches = arches
		}
		locked := map[string][]string{}
		for _, arch := range setArches {
			pkgs, err := alpine.New(alpine.AlpineOpts{
				Distro:   alpine.AlpineDistroWolfi,
				Packages: set.packages,
				Arch:     arch,
			}).Resolve(ctx)
			if err != nil {
				return nil, fmt.Errorf("resolve set %q for %s: %w", name, arch, err)
			}
			slices.Sort(pkgs)
			locked[arch] = pkgs
		}
		lock.Sets[name] = lockedSet{Packages: set.packages, Locked: locked}
	}
	out, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, err
	}
	return core.NewDirectory().
		WithNewFile("wolfi.lock.json", string(out)+"\n").
		File("wolfi.lock.json"), nil
}
