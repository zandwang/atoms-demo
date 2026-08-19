package compiler

import (
	"strings"

	"github.com/zand/atoms-demo/internal/domain"
)

func CompileGenerated(files domain.GeneratedFiles) (domain.CompiledArtifact, error) {
	if err := files.NormalizeAndValidate(); err != nil {
		return domain.CompiledArtifact{}, err
	}
	entry := strings.Join([]string{
		"<!doctype html>",
		"<html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
		"<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'none'; img-src data:; base-uri 'none'; form-action 'none'\">",
		"<title>Atoms Preview</title><style>" + files.CSS + "</style></head>",
		files.HTML,
		"<script>" + previewBridge + "\n" + files.JS + "</script>",
		"</html>",
	}, "\n")
	return domain.CompiledArtifact{
		EntryHTML: entry,
		HTML:      files.HTML,
		CSS:       files.CSS,
		JS:        files.JS,
		Manifest: domain.ArtifactManifest{
			Template: domain.TemplateCustom,
			Actions:  []string{"custom"},
			Checksum: artifactChecksum(entry),
		},
	}, nil
}
