package compiler

import (
	"strings"
	"testing"

	"github.com/zand/atoms-demo/internal/domain"
)

func TestCompileGeneratedPomodoro(t *testing.T) {
	files := domain.GeneratedFiles{
		HTML: `<main><h1>番茄钟</h1><output id="time">25:00</output><button id="start">开始</button></main>`,
		CSS:  `body { font-family: sans-serif; } output { display: block; font-size: 3rem; }`,
		JS:   `document.getElementById("start").addEventListener("click", () => { document.getElementById("time").textContent = "24:59"; });`,
	}
	artifact, err := Compile(domain.AppSpec{Files: &files})
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Manifest.Template != domain.TemplateCustom || artifact.Manifest.Checksum == "" {
		t.Fatalf("manifest = %#v", artifact.Manifest)
	}
	for _, expected := range []string{"番茄钟", "Content-Security-Policy", "atomsPreview", "24:59"} {
		if !strings.Contains(artifact.EntryHTML, expected) {
			t.Fatalf("entry HTML missing %q", expected)
		}
	}
}

func TestCompileGeneratedRejectsUnsafeSource(t *testing.T) {
	for _, files := range []domain.GeneratedFiles{
		{HTML: `<main>bad</main><script src="https://example.test/app.js"></script>`},
		{HTML: `<main>bad</main>`, JS: `fetch("https://example.test")`},
		{HTML: `<main>bad</main>`, CSS: `</style><script>alert(1)</script>`},
	} {
		if _, err := Compile(domain.AppSpec{Files: &files}); err == nil {
			t.Fatalf("unsafe files accepted: %#v", files)
		}
	}
}
