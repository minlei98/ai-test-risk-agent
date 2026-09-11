package jira

import (
	"encoding/json"
	"testing"
)

func TestDescriptionTextFromADF(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"doc",
		"version":1,
		"content":[
			{"type":"paragraph","content":[{"type":"text","text":"Acceptance Criteria:"}]},
			{"type":"paragraph","content":[{"type":"text","text":"Must deny cross-tenant access"}]}
		]
	}`)
	text := descriptionText(raw)
	if text == "" || text != "Acceptance Criteria:\nMust deny cross-tenant access" {
		t.Fatalf("unexpected adf text: %q", text)
	}
}

func TestDescriptionTextFromPlainString(t *testing.T) {
	raw := json.RawMessage(`"plain description"`)
	if descriptionText(raw) != "plain description" {
		t.Fatalf("unexpected plain text: %q", descriptionText(raw))
	}
}
