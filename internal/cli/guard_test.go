package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// findCmd resolves a command path against a freshly built command tree. Building
// the tree only needs the App pointer, not a live config store or cache, and the
// read-only guard runs before any command touches either.
func findCmd(t *testing.T, path string) *cobra.Command {
	t.Helper()
	root := NewRootCmd(&App{})
	cmd, _, err := root.Find(strings.Fields(path))
	if err != nil {
		t.Fatalf("find %q: %v", path, err)
	}
	if got := commandPathKey(cmd); got != path {
		t.Fatalf("resolved %q to %q", path, got)
	}
	return cmd
}

func TestReadOnlyGuardBlocksMutatingCommands(t *testing.T) {
	t.Setenv(allowWritesEnv, "")

	blocked := []string{
		"classes create",
		"classes update",
		"classes delete",
		"classwork create",
		"classwork edit",
		"classwork publish",
		"classwork schedule",
		"classwork delete",
		"stream post",
		"stream edit",
		"stream delete",
		"people invite",
		"people remove",
		"grades return",
		"submissions grade",
		"topics create",
		"topics edit",
		"topics delete",
		"topics move",
	}

	for _, path := range blocked {
		err := guardReadOnly(findCmd(t, path))
		if err == nil {
			t.Errorf("expected %q to be refused in read-only mode", path)
			continue
		}
		if !strings.Contains(err.Error(), allowWritesEnv) {
			t.Errorf("error for %q should name %s, got: %v", path, allowWritesEnv, err)
		}
	}
}

func TestReadOnlyGuardAllowsReadCommands(t *testing.T) {
	t.Setenv(allowWritesEnv, "")

	allowed := []string{
		"auth login",
		"auth status",
		"auth logout",
		"classes list",
		"classes show",
		"classwork list",
		"stream list",
		"submissions list",
		"submissions show",
		"people list",
		"grades list",
		"topics list",
		"to-do list",
		"calendar open",
		"handoff open",
	}

	for _, path := range allowed {
		if err := guardReadOnly(findCmd(t, path)); err != nil {
			t.Errorf("expected %q to be allowed, got: %v", path, err)
		}
	}
}

func TestReadOnlyGuardHonorsOptIn(t *testing.T) {
	t.Setenv(allowWritesEnv, "1")

	if err := guardReadOnly(findCmd(t, "classes delete")); err != nil {
		t.Fatalf("expected writes to be permitted with %s=1, got: %v", allowWritesEnv, err)
	}
}

// The guard is an allowlist, so a command added later is refused until someone
// consciously classifies it.
func TestReadOnlyGuardRefusesUnlistedCommand(t *testing.T) {
	t.Setenv(allowWritesEnv, "")

	root := NewRootCmd(&App{})
	classes, _, err := root.Find([]string{"classes"})
	if err != nil {
		t.Fatalf("find classes: %v", err)
	}
	newcomer := &cobra.Command{Use: "archive-everything"}
	classes.AddCommand(newcomer)

	if err := guardReadOnly(newcomer); err == nil {
		t.Fatal("expected an unlisted command to be refused by default")
	}
}

// Every mutating command must be reachable once the operator opts in; a typo in
// the allowlist that permanently disabled one would otherwise go unnoticed.
func TestReadOnlyGuardCoversEveryLeafCommand(t *testing.T) {
	t.Setenv(allowWritesEnv, "")

	root := NewRootCmd(&App{})
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			key := commandPathKey(child)
			if strings.HasPrefix(key, "completion") || strings.HasPrefix(key, "help") {
				continue
			}
			if !child.HasSubCommands() && !readOnlyAllowlist[key] {
				if err := guardReadOnly(child); err == nil {
					t.Errorf("%q is outside the allowlist but was not refused", key)
				}
			}
			walk(child)
		}
	}
	walk(root)
}
