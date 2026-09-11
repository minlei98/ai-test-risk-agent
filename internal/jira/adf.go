package jira

import (
	"encoding/json"
	"strings"
)

func descriptionText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var doc adfNode
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.plainText())
}

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

func (n adfNode) plainText() string {
	if n.Text != "" {
		return n.Text
	}
	var parts []string
	for _, child := range n.Content {
		part := child.plainText()
		if part == "" {
			continue
		}
		if n.Type == "paragraph" || child.Type == "paragraph" {
			parts = append(parts, part)
		} else {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}
