package auth

import (
	"sort"
	"strings"
)

// scopePrefix is the namespace every Google Classroom and userinfo scope lives
// under. Short names in --scopes are expanded against it.
const scopePrefix = "https://www.googleapis.com/auth/"

const (
	ScopeOpenID                      = "openid"
	ScopeUserEmail                   = "https://www.googleapis.com/auth/userinfo.email"
	ScopeUserProfile                 = "https://www.googleapis.com/auth/userinfo.profile"
	ScopeCoursesReadonly             = "https://www.googleapis.com/auth/classroom.courses.readonly"
	ScopeCourses                     = "https://www.googleapis.com/auth/classroom.courses"
	ScopeRostersReadonly             = "https://www.googleapis.com/auth/classroom.rosters.readonly"
	ScopeRosters                     = "https://www.googleapis.com/auth/classroom.rosters"
	ScopeAnnouncementsReadonly       = "https://www.googleapis.com/auth/classroom.announcements.readonly"
	ScopeAnnouncements               = "https://www.googleapis.com/auth/classroom.announcements"
	ScopeCourseWorkMeReadonly        = "https://www.googleapis.com/auth/classroom.coursework.me.readonly"
	ScopeCourseWorkMe                = "https://www.googleapis.com/auth/classroom.coursework.me"
	ScopeCourseWorkStudentsReadonly  = "https://www.googleapis.com/auth/classroom.coursework.students.readonly"
	ScopeCourseWorkStudents          = "https://www.googleapis.com/auth/classroom.coursework.students"
	ScopeTopicsReadonly              = "https://www.googleapis.com/auth/classroom.topics.readonly"
	ScopeTopics                      = "https://www.googleapis.com/auth/classroom.topics"
	ScopeCourseWorkMaterialsReadonly = "https://www.googleapis.com/auth/classroom.courseworkmaterials.readonly"
	ScopeCourseWorkMaterials         = "https://www.googleapis.com/auth/classroom.courseworkmaterials"
	ScopeDriveFile                   = "https://www.googleapis.com/auth/drive.file"
)

// DefaultReadScopes is deliberately narrower than the set of readonly scopes the
// API offers. Roster and all-students coursework scopes expose other people's
// names, email addresses and submitted work; nothing in the read path needs them,
// so they are opt-in via --scopes rather than granted on every login.
var DefaultReadScopes = []string{
	ScopeOpenID,
	ScopeUserEmail,
	ScopeCoursesReadonly,
	ScopeAnnouncementsReadonly,
	ScopeCourseWorkMeReadonly,
	ScopeTopicsReadonly,
	ScopeCourseWorkMaterialsReadonly,
}

var DefaultWriteScopes = []string{
	ScopeOpenID,
	ScopeUserEmail,
	ScopeUserProfile,
	ScopeCourses,
	ScopeRosters,
	ScopeAnnouncements,
	ScopeCourseWorkMe,
	ScopeCourseWorkStudents,
	ScopeTopics,
	ScopeCourseWorkMaterials,
	ScopeDriveFile,
}

// ExpandScope turns a short scope name such as "classroom.courses.readonly" into
// the full URL Google requires. Google rejects the whole authorization request
// with invalid_scope if a bare name is sent, so accepting both spellings is the
// difference between --scopes working and failing.
func ExpandScope(scope string) string {
	s := strings.TrimSpace(scope)
	if s == "" || s == ScopeOpenID {
		return s
	}
	if strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") {
		return s
	}
	return scopePrefix + s
}

func ExpandScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if expanded := ExpandScope(s); expanded != "" {
			out = append(out, expanded)
		}
	}
	return out
}

func NormalizeScopes(scopes []string) []string {
	uniq := map[string]struct{}{}
	for _, s := range scopes {
		s = ExpandScope(s)
		if s == "" {
			continue
		}
		uniq[s] = struct{}{}
	}
	out := make([]string, 0, len(uniq))
	for s := range uniq {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
