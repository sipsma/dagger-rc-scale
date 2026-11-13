package schema

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/dagger/dagger/util/gitutil"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/utils/ioutil"
)

// errRedirectAwayFromRefs reports a redirect of a probe somewhere other than
// a repository's refs, as Azure Repos redirects a private repository's
// probe to its sign-in host.
var errRedirectAwayFromRefs = errors.New("http redirect away from /info/refs")

// publicRemoteHTTPClient stops at a redirect away from the refs before
// contacting its target. go-git would follow it and only then reject the
// target, so the probe's answer would depend on reaching an unrelated host.
var publicRemoteHTTPClient = githttp.NewClient(&http.Client{
	Transport: gitHTTPTransport,
	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if !strings.HasSuffix(req.URL.Path, "/info/refs") {
			return errRedirectAwayFromRefs
		}
		return nil
	},
})

// Use the same transport as go-git's ListContext, but retain the advertisement
// before AllReferences guesses symbolic HEAD on servers without that capability.
// Git's ls-remote --symref reports only symbolic refs actually advertised.
func publicRemoteAdvertisement(ctx context.Context, remote *gitutil.GitURL) (_ *gitutil.Remote, rerr error) {
	endpoint, err := transport.NewEndpoint(remote.Remote())
	if err != nil {
		return nil, err
	}
	var transportClient = publicRemoteHTTPClient
	if endpoint.Protocol != "http" && endpoint.Protocol != "https" {
		transportClient, err = client.NewClient(endpoint)
		if err != nil {
			return nil, err
		}
	}
	session, err := transportClient.NewUploadPackSession(endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer ioutil.CheckClose(session, &rerr)
	advertised, err := session.AdvertisedReferencesContext(ctx)
	if err != nil {
		return nil, err
	}
	// Preserve ListContext's validation and visibility error classification.
	if _, err := advertised.AllReferences(); err != nil {
		return nil, err
	}
	return remoteFromAdvertisement(advertised), nil
}

func remoteFromAdvertisement(advertised *packp.AdvRefs) *gitutil.Remote {
	remote := &gitutil.Remote{
		Refs:    make([]*gitutil.Ref, 0, 1+len(advertised.References)+len(advertised.Peeled)),
		Symrefs: make(map[string]string),
	}
	if advertised.Head != nil {
		remote.Refs = append(remote.Refs, &gitutil.Ref{Name: "HEAD", SHA: advertised.Head.String()})
	}
	for name, hash := range advertised.References {
		remote.Refs = append(remote.Refs, &gitutil.Ref{Name: name, SHA: hash.String()})
	}
	for name, hash := range advertised.Peeled {
		remote.Refs = append(remote.Refs, &gitutil.Ref{Name: name + "^{}", SHA: hash.String()})
	}
	for _, symref := range advertised.Capabilities.Get(capability.SymRef) {
		if name, target, ok := strings.Cut(symref, ":"); ok {
			remote.Symrefs[name] = target
		}
	}
	// Git lists each peeled ref immediately after its tag. Stable sorting by
	// the unpeeled name keeps the tag first and preserves ls-remote's digest.
	slices.SortStableFunc(remote.Refs, func(a, b *gitutil.Ref) int {
		return cmp.Compare(strings.TrimSuffix(a.Name, "^{}"), strings.TrimSuffix(b.Name, "^{}"))
	})
	return remote
}
