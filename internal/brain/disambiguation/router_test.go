package disambiguation

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	t.Parallel()

	router, err := LoadFromFile(configPath(t))
	if err != nil {
		t.Fatalf("load disambiguation config: %v", err)
	}
	if router.Version() != 1 {
		t.Fatalf("unexpected disambiguation version: %d", router.Version())
	}

	gotGroups := router.Groups()
	wantGroups := append([]string(nil), requiredGroups...)
	slices.Sort(wantGroups)
	if !slices.Equal(gotGroups, wantGroups) {
		t.Fatalf("unexpected groups: got=%v want=%v", gotGroups, wantGroups)
	}
}

func TestResolveAcrossGroups(t *testing.T) {
	t.Parallel()

	router, err := LoadFromFile(configPath(t))
	if err != nil {
		t.Fatalf("load disambiguation config: %v", err)
	}

	tests := []struct {
		name    string
		input   ResolveInput
		wantID  string
		wantErr bool
	}{
		{
			name:   "apple notes alias always canonical",
			input:  ResolveInput{Group: "apple-notes", Text: "save this in apple notes"},
			wantID: "apple-notes-skill",
		},
		{
			name:   "notion canonical default",
			input:  ResolveInput{Group: "notion", Text: "add this to notion"},
			wantID: "better-notion",
		},
		{
			name:   "notion fallback when requested",
			input:  ResolveInput{Group: "notion", Text: "use legacy notion", PreferFallback: true},
			wantID: "notion",
		},
		{
			name:   "spotify cloud route",
			input:  ResolveInput{Group: "spotify", Text: "play my chill mix", DeploymentMode: "cloud"},
			wantID: "spotify-web-api",
		},
		{
			name:   "spotify local mac route",
			input:  ResolveInput{Group: "spotify", Text: "play music", DeploymentMode: "local_mac"},
			wantID: "spotify",
		},
		{
			name:   "spotify terminal route",
			input:  ResolveInput{Group: "spotify", Text: "queue this song", DeploymentMode: "terminal"},
			wantID: "spotify-player",
		},
		{
			name:   "spotify analytics route",
			input:  ResolveInput{Group: "spotify", Text: "show my top artists this month", DeploymentMode: "cloud"},
			wantID: "spotify-history",
		},
		{
			name:   "flight free tier route",
			input:  ResolveInput{Group: "flight-tracking", Text: "track AA100", UserTier: "free"},
			wantID: "flight-tracker",
		},
		{
			name:   "flight find route",
			input:  ResolveInput{Group: "flight-tracking", Text: "find flight from nyc to mia", UserTier: "pro"},
			wantID: "aerobase-skill",
		},
		{
			name:   "flight track route",
			input:  ResolveInput{Group: "flight-tracking", Text: "track AA100", UserTier: "pro"},
			wantID: "aviationstack-flight-tracker",
		},
		{
			name:   "healthkit canonical",
			input:  ResolveInput{Group: "healthkit", Text: "sync my healthkit data"},
			wantID: "healthkit-sync-apple",
		},
		{
			name:   "apple mail search route",
			input:  ResolveInput{Group: "apple-mail", Text: "search for invoices from last month"},
			wantID: "apple-mail-search",
		},
		{
			name:   "apple mail default route",
			input:  ResolveInput{Group: "apple-mail", Text: "reply to latest email"},
			wantID: "apple-mail",
		},
		{
			name:   "email send google route",
			input:  ResolveInput{Group: "email-send", Text: "send email", Preferences: UserPreferences{EmailProvider: "google"}},
			wantID: "google-workspace",
		},
		{
			name:   "email send microsoft route",
			input:  ResolveInput{Group: "email-send", Text: "send email", Preferences: UserPreferences{EmailProvider: "microsoft"}},
			wantID: "outlook",
		},
		{
			name:   "email send apple route",
			input:  ResolveInput{Group: "email-send", Text: "send email", Preferences: UserPreferences{EmailProvider: "apple"}},
			wantID: "apple-mail",
		},
		{
			name:   "email send imap route",
			input:  ResolveInput{Group: "email-send", Text: "send email", Preferences: UserPreferences{EmailProvider: "imap"}},
			wantID: "imap-email",
		},
		{
			name:   "email send unknown defaults to smtp",
			input:  ResolveInput{Group: "email-send", Text: "send email", Preferences: UserPreferences{EmailProvider: "none"}},
			wantID: "smtp-send",
		},
		{
			name:   "expense tracker orchestrator",
			input:  ResolveInput{Group: "expense-tracking", Text: "I spent $22 on lunch"},
			wantID: "smart-expense-tracker",
		},
		{
			name:   "package austrian post",
			input:  ResolveInput{Group: "package-tracking", TrackingNumber: "RR123456789AT"},
			wantID: "post-at",
		},
		{
			name:   "package 17track by carrier",
			input:  ResolveInput{Group: "package-tracking", TrackingNumber: "LP123456789CN", Carrier: "yunexpress"},
			wantID: "track17",
		},
		{
			name:   "package international default",
			input:  ResolveInput{Group: "package-tracking", TrackingNumber: "1Z999AA10123456784", Carrier: "ups"},
			wantID: "parcel-package-tracking",
		},
		{
			name:   "places navigate route",
			input:  ResolveInput{Group: "places-location", Text: "navigate to jfk airport"},
			wantID: "google-maps",
		},
		{
			name:   "places find all route",
			input:  ResolveInput{Group: "places-location", Text: "find all coffee shops in soho"},
			wantID: "spots",
		},
		{
			name:   "places rich nearby route",
			input:  ResolveInput{Group: "places-location", Text: "best sushi near me"},
			wantID: "goplaces",
		},
		{
			name:   "places simple nearby route",
			input:  ResolveInput{Group: "places-location", Text: "quick nearby coffee options"},
			wantID: "local-places",
		},
		{
			name:   "youtube summarize route",
			input:  ResolveInput{Group: "youtube", Text: "summarize this youtube video"},
			wantID: "youtube-summarizer",
		},
		{
			name:   "youtube download route",
			input:  ResolveInput{Group: "youtube", Text: "download transcript from this video"},
			wantID: "video-transcript-downloader",
		},
		{
			name:   "youtube default search route",
			input:  ResolveInput{Group: "youtube", Text: "search videos about vc funding"},
			wantID: "youtube-api",
		},
		{
			name:    "unknown group errors",
			input:   ResolveInput{Group: "unknown"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := router.Resolve(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if result.SkillID != tc.wantID {
				t.Fatalf("unexpected skill route: got=%s want=%s (reason=%s)", result.SkillID, tc.wantID, result.Reason)
			}
		})
	}
}

func configPath(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	return filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "config", "skill-disambiguation.yaml")
}
