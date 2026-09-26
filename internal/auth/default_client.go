package auth

// Upstream shipped a baked-in OAuth client belonging to the project author, so a
// default install routed every consent through a third party's Google Cloud
// project. That project controls the consent screen, its verification status and
// its publishing state, none of which a user of this tool can see or audit. This
// fork ships no default client: supply your own.
//
// Set one of:
//   - GC_OAUTH_CLIENT_ID / GC_OAUTH_CLIENT_SECRET
//   - gc auth login --client-id ... --client-secret ...
//
// Or inject at build time with:
//
//	-ldflags="-X github.com/timothy/gc-cli/internal/auth.DefaultOAuthClientID=... \
//	          -X github.com/timothy/gc-cli/internal/auth.DefaultOAuthClientSecret=..."
var (
	DefaultOAuthClientID     = ""
	DefaultOAuthClientSecret = ""
)
