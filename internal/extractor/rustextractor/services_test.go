package rustextractor

import (
	"strings"
	"testing"

	"github.com/dshills/atlas/internal/extractor/commentfilter"
)

func TestExtractServices_Reqwest(t *testing.T) {
	content := `use reqwest;

async fn fetch_data() {
    let client = reqwest::Client::new();
    let resp = reqwest::get("https://api.example.com").await?;
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractServices(content, lines, codeLines)

	if len(arts) < 2 {
		t.Fatalf("expected at least 2 external_service artifacts, got %d", len(arts))
	}
	if arts[0].ArtifactKind != "external_service" {
		t.Errorf("expected artifact kind 'external_service', got %q", arts[0].ArtifactKind)
	}
	if len(refs) < 2 {
		t.Fatalf("expected at least 2 invokes_external_api refs, got %d", len(refs))
	}
	if refs[0].ReferenceKind != "invokes_external_api" {
		t.Errorf("expected reference kind 'invokes_external_api', got %q", refs[0].ReferenceKind)
	}
}

func TestExtractServices_TokioSpawn(t *testing.T) {
	content := `use tokio;

async fn run() {
    tokio::spawn(async {
        do_work().await;
    });
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractServices(content, lines, codeLines)

	// Should produce background_job artifact, no reference
	hasJob := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "tokio::spawn" {
			hasJob = true
			break
		}
	}
	if !hasJob {
		t.Error("expected background_job artifact for tokio::spawn")
	}
	// Background jobs should not produce references
	for _, r := range refs {
		if r.ReferenceKind == "invokes_external_api" {
			t.Error("background jobs should not produce invokes_external_api references")
		}
	}
}

func TestExtractServices_ThreadSpawn(t *testing.T) {
	content := `use std::thread;

fn run() {
    thread::spawn(|| {
        heavy_computation();
    });
}
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "rust")

	_, arts := extractServices(content, lines, codeLines)

	hasJob := false
	for _, a := range arts {
		if a.ArtifactKind == "background_job" && a.Name == "thread::spawn" {
			hasJob = true
			break
		}
	}
	if !hasJob {
		t.Error("expected background_job artifact for thread::spawn")
	}
}

func TestExtractServices_CommentedOut(t *testing.T) {
	content := `// reqwest::get("https://hidden.example.com")
let client = reqwest::Client::new();
`
	lines := strings.Split(content, "\n")
	codeLines := commentfilter.LineFilter(content, "rust")

	refs, arts := extractServices(content, lines, codeLines)

	// Only the non-commented line should match
	if len(arts) != 1 {
		t.Fatalf("expected 1 artifact (commented excluded), got %d", len(arts))
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref (commented excluded), got %d", len(refs))
	}
}
