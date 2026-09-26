package auth

import (
	"slices"
	"testing"
)

// Google rejects the entire authorization request with invalid_scope if a bare
// scope name reaches it, which is what upstream's own --scopes example produced.
func TestExpandScopeAddsPrefixToShortNames(t *testing.T) {
	got := ExpandScope("classroom.courses.readonly")
	want := ScopeCoursesReadonly
	if got != want {
		t.Fatalf("ExpandScope() = %q, want %q", got, want)
	}
}

func TestExpandScopeLeavesFullURLsAlone(t *testing.T) {
	if got := ExpandScope(ScopeCourseWorkMeReadonly); got != ScopeCourseWorkMeReadonly {
		t.Fatalf("ExpandScope() rewrote a full URL: %q", got)
	}
}

func TestExpandScopeLeavesOpenIDAlone(t *testing.T) {
	if got := ExpandScope(ScopeOpenID); got != ScopeOpenID {
		t.Fatalf("ExpandScope(openid) = %q, want %q", got, ScopeOpenID)
	}
}

func TestNormalizeScopesExpandsAndDeduplicates(t *testing.T) {
	got := NormalizeScopes([]string{
		"classroom.courses.readonly",
		ScopeCoursesReadonly,
		"",
		"  classroom.coursework.me.readonly  ",
	})
	want := []string{ScopeCoursesReadonly, ScopeCourseWorkMeReadonly}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeScopes() = %v, want %v", got, want)
	}
}

// Roster and all-students coursework scopes expose other people's names, email
// addresses and submitted work. Nothing in the read path needs them.
func TestDefaultReadScopesExcludeOtherPeoplesData(t *testing.T) {
	for _, unwanted := range []string{
		ScopeRostersReadonly,
		ScopeCourseWorkStudentsReadonly,
	} {
		if slices.Contains(DefaultReadScopes, unwanted) {
			t.Errorf("DefaultReadScopes should not request %s", unwanted)
		}
	}
}

func TestDefaultReadScopesAreAllReadOnly(t *testing.T) {
	for _, scope := range DefaultWriteScopes {
		if scope == ScopeOpenID || scope == ScopeUserEmail || scope == ScopeUserProfile {
			continue
		}
		if slices.Contains(DefaultReadScopes, scope) {
			t.Errorf("DefaultReadScopes contains the write scope %s", scope)
		}
	}
}

// This fork ships no default OAuth client; consent must go through the operator's
// own Google Cloud project, not a third party's.
func TestNoBakedInOAuthClient(t *testing.T) {
	if DefaultOAuthClientID != "" {
		t.Errorf("DefaultOAuthClientID should be empty, got %q", DefaultOAuthClientID)
	}
	if DefaultOAuthClientSecret != "" {
		t.Error("DefaultOAuthClientSecret should be empty")
	}
}
