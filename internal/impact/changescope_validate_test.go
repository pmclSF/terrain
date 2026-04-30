package impact

import "testing"

func TestValidateBaseRef(t *testing.T) {
	t.Parallel()

	ok := []string{
		"",
		"HEAD",
		"HEAD~1",
		"HEAD^",
		"HEAD^^",
		"main",
		"origin/main",
		"feature/login-redux",
		"release-1.2.3",
		"v1.2.3",
		"v1.2.3-rc.1",
		"abcdef0123456789",
		"user@email/branch", // @ is allowed for username-prefixed remotes
	}
	for _, ref := range ok {
		if err := ValidateBaseRef(ref); err != nil {
			t.Errorf("ValidateBaseRef(%q) returned %v; want nil", ref, err)
		}
	}

	bad := []string{
		"; rm -rf /",          // shell injection
		"main && echo pwned",  // chained command
		"main | cat",          // pipe
		"main`echo`",          // backticks
		"main$(echo)",         // command substitution
		"main\nchaining",      // newline injection
		"main with spaces",    // whitespace
		"@{-1}",               // reflog selector
		"main:secret/path",    // ref:path form
		"--upload-pack=evil",  // option injection
		">file",               // redirect
	}
	for _, ref := range bad {
		if err := ValidateBaseRef(ref); err == nil {
			t.Errorf("ValidateBaseRef(%q) returned nil; want error", ref)
		}
	}
}

func TestChangeScopeFromGitDiff_RejectsInvalidRef(t *testing.T) {
	t.Parallel()

	if _, err := ChangeScopeFromGitDiff(".", "; rm -rf /"); err == nil {
		t.Errorf("expected error for shell-injection base ref")
	}
}
