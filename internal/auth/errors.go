package auth

import "fmt"

var ErrNoToken = fmt.Errorf("no saved OAuth token found")
var ErrMissingClientID = fmt.Errorf(
	"missing OAuth client id. This fork ships no default client, so create a Desktop OAuth client " +
		"in your own Google Cloud project and set GC_OAUTH_CLIENT_ID and GC_OAUTH_CLIENT_SECRET " +
		"(or pass --client-id / --client-secret to gc auth login)")

type ScopesRequiredError struct {
	Missing []string
}

func (e ScopesRequiredError) Error() string {
	return fmt.Sprintf("missing OAuth scopes: %v", e.Missing)
}
