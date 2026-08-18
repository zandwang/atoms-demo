package compiler

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zand/atoms-demo/internal/domain"
)

func makeArtifact(spec domain.AppSpec, html, css, appJS string, actions []string) (domain.CompiledArtifact, error) {
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return domain.CompiledArtifact{}, fmt.Errorf("encode application specification: %w", err)
	}
	js := previewBridge + "\n" + appJS
	entryHTML := strings.Join([]string{
		"<!doctype html>",
		"<html lang=\"zh-CN\">",
		"<head>",
		"<meta charset=\"utf-8\">",
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
		"<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'none'; img-src data:; base-uri 'none'; form-action 'none'\">",
		"<title>Atoms Preview</title>",
		"<style>" + css + "</style>",
		"</head>",
		html,
		"<script id=\"atoms-spec\" type=\"application/json\">" + string(specJSON) + "</script>",
		"<script>" + js + "</script>",
		"</html>",
	}, "\n")
	return domain.CompiledArtifact{
		EntryHTML: entryHTML,
		HTML:      html,
		CSS:       css,
		JS:        js,
		Manifest: domain.ArtifactManifest{
			Template: spec.TemplateName(),
			Actions:  actions,
			Checksum: artifactChecksum(entryHTML),
		},
	}, nil
}

// previewBridge is the only communication channel between the opaque sandbox
// and the workbench. It carries serializable app state, never cookies or DOM.
const previewBridge = `(() => {
  "use strict";
  let versionID = "";
  let restore = () => {};

  function receive(event) {
    if (event.source !== window.parent || !event.data || typeof event.data !== "object") return;
    const data = event.data;
    if ((data.type !== "atoms-preview:init" && data.type !== "atoms-preview:restore") || typeof data.versionId !== "string") return;
    versionID = data.versionId;
    if (data.state && typeof data.state === "object") restore(data.state);
    window.parent.postMessage({ type: "atoms-preview:ready", versionId: versionID }, "*");
  }

  window.addEventListener("message", receive);
  window.atomsPreview = Object.freeze({
    onRestore(callback) {
      restore = typeof callback === "function" ? callback : () => {};
    },
    publish(state) {
      if (!versionID) return;
      window.parent.postMessage({ type: "atoms-preview:state-change", versionId: versionID, state }, "*");
    }
  });
})();`
