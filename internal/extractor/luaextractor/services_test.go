package luaextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_HTTPClient(t *testing.T) {
	content := `local http = require("socket.http")
local body = http.request("http://example.com/api")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) < 1 {
		t.Fatalf("expected at least 1 service ref, got %d", len(refs))
	}

	found := false
	for _, r := range refs {
		if r.ReferenceKind == "invokes_external_api" {
			found = true
			if r.Confidence != "heuristic" {
				t.Errorf("expected confidence 'heuristic', got %q", r.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected invokes_external_api reference")
	}

	hasExternalService := false
	for _, a := range arts {
		if a.ArtifactKind == "external_service" {
			hasExternalService = true
		}
	}
	if !hasExternalService {
		t.Error("expected external_service artifact")
	}
}

func TestExtractServices_SocketHTTP(t *testing.T) {
	content := `local result = socket.http("http://api.example.com")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 service ref, got %d", len(refs))
	}
	if refs[0].RawTargetText != "socket.http" {
		t.Errorf("expected raw target 'socket.http', got %q", refs[0].RawTargetText)
	}
	if arts[0].ArtifactKind != "external_service" {
		t.Errorf("expected artifact kind 'external_service', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractServices_BackgroundJob(t *testing.T) {
	content := `ngx.timer.at(0, function()
    -- background work
end)
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 background job ref, got %d", len(refs))
	}
	if refs[0].RawTargetText != "ngx.timer.at" {
		t.Errorf("expected raw target 'ngx.timer.at', got %q", refs[0].RawTargetText)
	}

	if len(arts) != 1 {
		t.Fatalf("expected 1 background job artifact, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "background_job" {
		t.Errorf("expected artifact kind 'background_job', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractServices_CopasThread(t *testing.T) {
	content := `copas.addthread(function()
    -- async work
end)
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, arts := extractServices(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].RawTargetText != "copas.addthread" {
		t.Errorf("expected raw target 'copas.addthread', got %q", refs[0].RawTargetText)
	}
	if arts[0].ArtifactKind != "background_job" {
		t.Errorf("expected artifact kind 'background_job', got %q", arts[0].ArtifactKind)
	}
}

func TestExtractServices_CommentedOut(t *testing.T) {
	content := `-- http.request("http://hidden.com")
local body = http.request("http://visible.com")
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "lua")

	refs, _ := extractServices(content, lines, codeLines)

	if len(refs) != 1 {
		t.Fatalf("expected 1 service ref (commented excluded), got %d", len(refs))
	}
}
