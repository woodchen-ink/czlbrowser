package backend

import (
	"testing"
)

func TestCappedLogBufferKeepsTail(t *testing.T) {
	t.Parallel()

	buf := newCappedLogBuffer(5)
	_, _ = buf.Write([]byte("1234"))
	_, _ = buf.Write([]byte("5678"))
	if got := buf.String(); got != "45678" {
		t.Fatalf("unexpected capped output: %q", got)
	}
}

func TestSummarizeProcessOutput(t *testing.T) {
	t.Parallel()

	raw := "line1\n\nline2\nline3\nline4"
	got := summarizeProcessOutput(raw, 2, 20)
	if got != "line3 | line4" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestParseProxyLocaleFromHealthJSON(t *testing.T) {
	t.Parallel()

	raw := `{"timezone":"Europe/Berlin","countryCode":"DE","rawData":{"timezone":"Europe/Berlin"}}`
	timezone, countryCode, ok := parseProxyLocaleFromHealthJSON(raw)
	if !ok {
		t.Fatalf("expected parse ok")
	}
	if timezone != "Europe/Berlin" {
		t.Fatalf("unexpected timezone: %q", timezone)
	}
	if countryCode != "DE" {
		t.Fatalf("unexpected countryCode: %q", countryCode)
	}
}

func TestUpsertKeyValueArg(t *testing.T) {
	t.Parallel()

	args := []string{"--disable-sync", "--lang=zh-CN", "--timezone", "Asia/Shanghai"}
	out := upsertKeyValueArg(args, "--lang", "de-DE")
	out = upsertKeyValueArg(out, "--timezone", "Europe/Berlin")

	if out[1] != "--lang=de-DE" {
		t.Fatalf("lang not replaced: %#v", out)
	}
	foundTZ := false
	for _, arg := range out {
		if arg == "--timezone=Europe/Berlin" {
			foundTZ = true
			break
		}
	}
	if !foundTZ {
		t.Fatalf("timezone not upserted: %#v", out)
	}
}

func TestExtractSwitchValueArg(t *testing.T) {
	t.Parallel()

	args := []string{"--lang=zh-CN", "--timezone=America/New_York", "--flag", "--timezone", "Europe/Berlin"}
	filtered, timezone := extractSwitchValueArg(args, "--timezone")
	if timezone != "Europe/Berlin" {
		t.Fatalf("unexpected timezone: %q", timezone)
	}
	for _, arg := range filtered {
		if arg == "--timezone=America/New_York" || arg == "--timezone" || arg == "Europe/Berlin" {
			t.Fatalf("timezone args should have been removed: %#v", filtered)
		}
	}
}

func TestShouldManageStealthTarget(t *testing.T) {
	t.Parallel()

	if !shouldManageStealthTarget("page") {
		t.Fatalf("page target should be managed")
	}
	if !shouldManageStealthTarget("background_page") {
		t.Fatalf("background_page target should be managed")
	}
	if shouldManageStealthTarget("service_worker") {
		t.Fatalf("service_worker target should not be managed")
	}
}

func TestParseAttachedTargetEvent(t *testing.T) {
	t.Parallel()

	event := cdpResponse{
		Method: "Target.attachedToTarget",
		Params: map[string]any{
			"sessionId": "session-1",
			"targetInfo": map[string]any{
				"targetId": "target-1",
				"type":     "page",
			},
		},
	}

	sessionID, targetID, targetType, ok := parseAttachedTargetEvent(event)
	if !ok {
		t.Fatalf("expected attachedToTarget event to parse")
	}
	if sessionID != "session-1" || targetID != "target-1" || targetType != "page" {
		t.Fatalf("unexpected parse result: %q %q %q", sessionID, targetID, targetType)
	}
}

func TestDefaultLanguageByCountryCode(t *testing.T) {
	t.Parallel()

	if got := defaultLanguageByCountryCode("DE"); got != "de-DE" {
		t.Fatalf("unexpected language for DE: %q", got)
	}
	if got := defaultLanguageByCountryCode("GB"); got != "en-GB" {
		t.Fatalf("unexpected language for GB: %q", got)
	}
	if got := defaultLanguageByCountryCode(""); got != "en-US" {
		t.Fatalf("unexpected fallback language: %q", got)
	}
}
