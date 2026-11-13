package main

import (
	"context"
	"strconv"
	"strings"

	"dagger/cli-dev/internal/dagger"
	gomod "dagger/cli-dev/internal/dagger/clients/go"

	"dagger.io/dagger/core"
	"github.com/containerd/platforms"
	"golang.org/x/mod/semver"
)

func New(
	ctx context.Context,

	// +optional
	runnerHost string,

	// +optional
	// +defaultPath="/"
	// +ignore=[
	//   "*",
	//   ".*",
	//   "!cmd/dagger/*",
	//   "!internal/cmd/dagger/**",
	//   "!**/go.sum",
	//   "!**/go.mod",
	//   "!**/*.go",
	//   "!**/VERSION",
	//   "!vendor/**/*",
	//   "!**.graphql",
	//   "!.changes",
	//   "!LICENSE",
	//   "!install.sh",
	//   "!install.ps1",
	//   "!**/*.sql",
	//   "!core/prompts/*.md", "!core/sdk/sdkmeta/sdks.json"
	// ]
	source *dagger.Directory,

	// Base image for go build environment
	// +optional
	base *dagger.Container,

	// Version of the Dagger CLI being built. Surfaced as CliDev.Version and
	// consumed by the publish flow (goreleaser ENGINE_VERSION, S3 paths,
	// semver release compatibility). The built binary self-reports its own version
	// from the embedded internal/version/VERSION file regardless of what's
	// passed here, but this decides which engine the binary provisions by
	// default: a valid semver means a tag build (embedded VERSION already
	// matches, enforced by the publish workflow guard); anything else is a
	// commit build, whose default engine tag is pinned to the commit.
	// +optional
	version string,

	// The VCS commit to stamp into the binary, which also pins the default
	// engine tag of a build whose version isn't semver. Without it the binary
	// is unstamped. A stamped binary changes with every commit, so only a
	// published CLI carries one; a CLI a check or test builds is the same
	// bytes at every commit of the same source.
	// +optional
	vcsCommit string,

	// Whether the stamped commit's tree had uncommitted changes, paired with
	// vcsCommit.
	// +optional
	vcsDirty bool,
) (*CliDev, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// FIXME: this go builder config is duplicated with engine build
	// move into a shared engine/builder module
	var values []string
	if vcsCommit != "" {
		values = append(values,
			"github.com/dagger/dagger/internal/version/buildinfo.InjectedVCS=git",
			"github.com/dagger/dagger/internal/version/buildinfo.InjectedVCSRevision="+vcsCommit,
			"github.com/dagger/dagger/internal/version/buildinfo.InjectedVCSModified="+strconv.FormatBool(vcsDirty),
		)
	}
	if runnerHost != "" {
		values = append(values, "github.com/dagger/dagger/internal/cmd/dagger.RunnerHost="+runnerHost)
	}

	if !semver.IsValid(version) && vcsCommit != "" {
		values = append(values, "github.com/dagger/dagger/engine.Tag="+vcsCommit)
	}

	return &CliDev{
		Version: version,
		Tag:     version,
		Source:  source,
		Base:    base,
		Values:  values,
	}, nil
}

// Develop the Dagger CLI
type CliDev struct {
	Version string
	Tag     string

	Source *dagger.Directory // +private
	// The Go build's base container; dagger/go's own Go image when unset.
	Base *dagger.Container // +private
	// Definitions stamped into the binary with -ldflags -X.
	Values []string // +private
}

// Build the dagger CLI binary for a single platform
func (cli CliDev) Binary(
	// +optional
	platform dagger.Platform,
) *dagger.File {
	// The source is the build's workspace, so the build is keyed by its
	// content and never by the caller's session.
	ws := cli.Source.AsWorkspace()
	ldflags := []string{"-s", "-w"}
	for _, val := range cli.Values {
		ldflags = append(ldflags, "-X '"+val+"'")
	}
	pkg := gomod.New(gomod.GoOpts{
		Version:    goVersion,
		Base:       cli.Base,
		BuildFlags: []string{"-ldflags=" + strings.Join(ldflags, " ")},
	}).Module(ws, "cmd/dagger").Packages(ws).Get("cmd/dagger")
	if platform == "" {
		return pkg.Binary(ws)
	}
	// dagger/go keys a package's platforms by GOOS/GOARCH.
	p := platforms.Normalize(platforms.MustParse(string(platform)))
	return pkg.Platforms().Get(p.OS + "/" + p.Architecture).Binary(ws)
}

// The Go release the CLI is built with.
const goVersion = "1.27"

// Build dev CLI binaries
// TODO: remove this
func (cli *CliDev) DevBinaries(
	// +optional
	platform dagger.Platform,
) *dagger.Directory {
	p := platforms.MustParse(string(platform))
	bin := cli.Binary(platform)
	binName := "dagger"
	if p.OS == "windows" {
		binName += ".exe"
	}
	dir := core.NewDirectory().WithFile(binName, bin)
	if p.OS != "linux" {
		p2 := p
		p2.OS = "linux"
		p2.OSFeatures = nil
		p2.OSVersion = ""
		dir = dir.WithFile("dagger-linux", cli.Binary(dagger.Platform(platforms.Format(p2))))
	}
	return dir
}
