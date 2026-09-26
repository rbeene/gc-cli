package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// This binary can delete courses, delete coursework, remove students, grade and
// turn in submissions. When it is driven by automation rather than by a person
// typing, that blast radius is unacceptable, so writes are refused unless the
// operator opts in explicitly.
//
// The gate is an allowlist, not a denylist: a command added later is refused by
// default rather than silently permitted.
const allowWritesEnv = "GC_ALLOW_WRITES"

// readOnlyAllowlist holds command paths (root command name stripped) that only
// read Classroom state or act locally.
var readOnlyAllowlist = map[string]bool{
	"":                 true,
	"auth":             true,
	"auth login":       true,
	"auth status":      true,
	"auth logout":      true,
	"classes":          true,
	"classes list":     true,
	"classes show":     true,
	"classwork":        true,
	"classwork list":   true,
	"stream":           true,
	"stream list":      true,
	"submissions":      true,
	"submissions list": true,
	"submissions show": true,
	"people":           true,
	"people list":      true,
	"grades":           true,
	"grades list":      true,
	"topics":           true,
	"topics list":      true,
	"to-do":            true,
	"to-do list":       true,
	"calendar":         true,
	"calendar open":    true,
	"handoff":          true,
	"handoff open":     true,
}

// alwaysAllowedRoots are cobra's own meta commands, which never touch the API.
var alwaysAllowedRoots = []string{"help", "completion"}

func writesAllowed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(allowWritesEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// commandPathKey renders the invoked command as a space-joined path with the
// root command name removed, so the key is stable no matter what the binary is
// named on disk.
func commandPathKey(cmd *cobra.Command) string {
	parts := make([]string, 0, 3)
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

// guardReadOnly refuses commands that can change Classroom state unless the
// operator set GC_ALLOW_WRITES.
func guardReadOnly(cmd *cobra.Command) error {
	if writesAllowed() {
		return nil
	}
	key := commandPathKey(cmd)
	for _, root := range alwaysAllowedRoots {
		if key == root || strings.HasPrefix(key, root+" ") {
			return nil
		}
	}
	if readOnlyAllowlist[key] {
		return nil
	}
	return fmt.Errorf(
		"refusing to run %q: this build is read-only because the command can change Classroom data. Set %s=1 to allow writes",
		key, allowWritesEnv,
	)
}
