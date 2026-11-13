package core

import (
	"context"
	"crypto/rand"
	"strings"

	"dagger.io/dagger/core"
	"github.com/dagger/testctx"
	"github.com/stretchr/testify/require"
)

// A Go module's runtime build finds the dependency calls for what generated
// code imports (distconsts.GoSDKSharedImports) whole on an engine that has
// built another module, though the two import different packages: only the
// call for the second module's own extra package runs. Without the shared
// calls, its one dependency call would differ from the first module's, and
// run.
func (GoSuite) TestRuntimeSharesDependencyCallsAcrossModules(ctx context.Context, t *testctx.T) {
	c := connect(ctx, t)
	engine, err := devEngineContainerAsService(telemetrySplitEngineWithoutCloud(c, devEngineContainer(c))).Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = engine.Stop(ctx) })
	cli := daggerCliFile(t, c)

	// call loads a module of name importing imports on the engine, in a new
	// session, and returns the cache outcomes of its dependency calls.
	call := func(name, imports, use string) []string {
		cloud := newTelemetrySplitCloud(t, c)
		_, err := telemetrySplitClient(ctx, t, c, cli, engine, cloud).
			WithWorkdir("/"+name).
			WithNewFile("dagger.json", `{"name":"`+name+`","engineVersion":"latest","sdk":{"source":"go"}}`).
			WithNewFile("main.go", "package main\n\n"+imports+"\n\ntype "+strings.ToUpper(name[:1])+name[1:]+" struct{}\n\nfunc (m *"+strings.ToUpper(name[:1])+name[1:]+") Hello() string { return "+use+" }\n").
			WithEnvVariable("CACHEBUSTER", rand.Text()).
			WithExec([]string{"/bin/dagger", "call", "-m", ".", "hello"}, core.ContainerWithExecOpts{DisableDaggerInDagger: true}).
			Stdout(ctx)
		require.NoError(t, err)
		type span struct {
			Name  string         `json:"name"`
			Attrs map[string]any `json:"attrs"`
		}
		var outcomes []string
		for _, s := range readTelemetrySplitLines[span](ctx, t, cloud, "v1/traces.json.spans") {
			if !strings.HasSuffix(s.Name, "Gobuild.dependencies") {
				continue
			}
			if outcome, _ := s.Attrs["dagger.io/cache.outcome"].(string); outcome != "" {
				outcomes = append(outcomes, outcome)
			}
		}
		t.Logf("%s's dependency call outcomes: %v", name, outcomes)
		return outcomes
	}

	call("bare", "", `"hi"`)
	// net/http/httptest is outside what generated code imports and reaches.
	outcomes := call("extra", `import "net/http/httptest"`, "httptest.DefaultRemoteAddr")
	var hits, ran int
	for _, outcome := range outcomes {
		if outcome == "hit" {
			hits++
		} else {
			ran++
		}
	}
	require.Equal(t, 2, hits, "the standard library's shared call and the other shared call are the first module's")
	require.Equal(t, 1, ran, "only the call for the second module's own extra package runs")
}
