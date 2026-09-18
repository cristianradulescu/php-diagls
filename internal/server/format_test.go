package server

import (
	"testing"

	"go.lsp.dev/protocol"
)

func TestFullDocumentEdit(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		formatted string
		wantEdits int
		wantEnd   protocol.Position
	}{
		{name: "identical content yields no edits", content: "<?php\n", formatted: "<?php\n", wantEdits: 0},
		{name: "trailing newline ends at empty last line", content: "<?php\n$a=1;\n", formatted: "<?php\n$a = 1;\n", wantEdits: 1, wantEnd: protocol.Position{Line: 2, Character: 0}},
		{name: "no trailing newline ends at last char", content: "<?php\n$a=1;", formatted: "<?php\n$a = 1;", wantEdits: 1, wantEnd: protocol.Position{Line: 1, Character: 5}},
		{name: "empty content", content: "", formatted: "<?php\n", wantEdits: 1, wantEnd: protocol.Position{Line: 0, Character: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edits := fullDocumentEdit(tt.content, tt.formatted)
			if len(edits) != tt.wantEdits {
				t.Fatalf("got %d edits, want %d", len(edits), tt.wantEdits)
			}
			if tt.wantEdits == 0 {
				return
			}
			if edits[0].Range.Start != (protocol.Position{}) {
				t.Errorf("start = %+v, want 0:0", edits[0].Range.Start)
			}
			if edits[0].Range.End != tt.wantEnd {
				t.Errorf("end = %+v, want %+v", edits[0].Range.End, tt.wantEnd)
			}
			if edits[0].NewText != tt.formatted {
				t.Errorf("NewText = %q, want %q", edits[0].NewText, tt.formatted)
			}
		})
	}
}
