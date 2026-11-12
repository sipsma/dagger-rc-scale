package main

// The package sets dagger/dagger's modules install, by name. After changing
// one, run Refresh to resolve it into wolfi.lock.json. A set that installs Go
// carries the Go version in its name, so callers ask for the set matching
// their Go version and fail until it exists.
var sets = map[string]packageSet{
	// engine-dev's Go environment for working on dagger/dagger: the Go
	// toolchain, a C toolchain, and what the engine's tests use
	"go-env-1.27": {packages: []string{
		"go-1.27",
		"ca-certificates",
		// a C toolchain, for cgo and the race detector
		"build-base",
		"git",
		// protoc, at the version dagger/dagger's own protobuf code needs
		"protobuf~35",
		"protobuf-dev~35",
		// the engine's tests exercise real client-managed agents and
		// encrypted key loading
		"openssh-client",
		"curl",
	}},

	// engine-dev's race-detector builds. A cgo binary links the C library of
	// the image it is built in, so it is built where it runs: wolfi.
	"go-race-1.27": {packages: []string{"go-1.27", "build-base"}},

	// The engine image's base
	"engine": {packages: enginePackages},
	// The engine image's base with GPU support, which is amd64 only
	"engine-gpu": {
		packages: append(append([]string{}, enginePackages...),
			"nvidia-driver", "nvidia-tools",
			"nvidia-container-toolkit",
			// Wolfi packages the OCI prestart hook separately from the toolkit.
			"nvidia-container-toolkit-nvidia-container-runtime-hook",
		),
		arches: []string{"amd64"},
	},

	// engine-dev's sshfs build. Wolfi's sshfs package currently lags upstream,
	// so the engine builds the known-good release from source.
	"sshfs-build": {packages: []string{
		"build-base",
		"ca-certificates-bundle",
		"coreutils",
		"fuse3-dev",
		"glib-dev",
		"meson",
	}},

	// engine-dev's check of a built binary's architecture
	"file": {packages: []string{"file"}},

	// A plain container: only busybox, which the Wolfi module always installs
	"busybox": {packages: []string{}},

	// dagger-dev's base for tasks such as the playground: basic shell tools
	"dagger-dev-base": {packages: []string{"apk-tools", "git", "ripgrep"}},

	// dagger-dev's Go tools and system dependencies for generating Dagger
	// source files
	"dagger-dev-go": {packages: []string{
		// FIXME: infer go version from our go.mod (ideally dagger.io/go has a utility for it)
		"go-1.27",
		"ca-certificates",
		"git",
		"build-base",
		"protobuf~35",
		"protobuf-dev~35",
		"clang-22",
		"lld",
		"libbpf-dev",
	}},

	// python-client-dev's base
	"python-client": {packages: []string{"libgcc"}},
}

var enginePackages = []string{
	"ca-certificates",
	"mount", "umount", "posix-libc-utils", "coreutils",
	// for the entrypoint to give the engine its own cgroup namespace
	"util-linux-misc",
	// for git
	"git", "openssh-client",
	// for SSHFS-backed volumes
	"fuse3", "glib",
	// for compression/decompression, containerd prefers igzip from the isa-l package as it's fastest
	"isa-l", "pigz", "xz",
	// for CNI (use nft variants for compatibility with kernels lacking legacy xtables)
	"nftables", "iptables-legacy", "dnsmasq",
	// for Kata Containers integration
	"e2fsprogs",
	// for Directory.search
	"ripgrep",
	// for dbs
	"sqlite",
}
