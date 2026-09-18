package diagnostics

import (
	"reflect"
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/config"
	"go.lsp.dev/protocol"
)

func rng(line, startChar, endChar uint32) protocol.Range {
	return protocol.Range{
		Start: protocol.Position{Line: line, Character: startChar},
		End:   protocol.Position{Line: line, Character: endChar},
	}
}

func TestPhpCsFixer_parseDiffForDiagnostics(t *testing.T) {
	dp := NewPhpCsFixer(config.DiagnosticsProvider{})

	tests := []struct {
		name string
		diff string
		want []protocol.Range
	}{
		{
			name: "empty diff",
			diff: "",
			want: nil,
		},
		{
			name: "removed line at hunk start",
			diff: "--- a\n+++ b\n@@ -1,3 +1,3 @@\n-line 1\n+LINE 1\n line 2\n line 3\n",
			want: []protocol.Range{rng(0, 0, 6)},
		},
		{
			name: "hunk count is not a column",
			// A 7-line hunk used to yield a start column of 6 (count - 1).
			diff: "--- a\n+++ b\n@@ -10,7 +10,7 @@\n line\n-$x = 1;\n+$x = 1 ;\n line\n",
			want: []protocol.Range{rng(10, 0, 7)},
		},
		{
			name: "indented line starts at code, ends at raw line length",
			diff: "--- a\n+++ b\n@@ -1,3 +1,3 @@\n line\n-        $a = array();\n+        $a = [];\n line\n",
			want: []protocol.Range{rng(1, 8, 21)},
		},
		{
			name: "multiple changes in one hunk track original lines",
			diff: "--- a\n+++ b\n@@ -1,5 +1,5 @@\n ctx\n-a\n+b\n ctx2\n-c\n+d\n",
			want: []protocol.Range{rng(1, 0, 1), rng(3, 0, 1)},
		},
		{
			name: "pure insertion is a zero-width range at insertion point",
			diff: "--- a\n+++ b\n@@ -1,2 +1,3 @@\n line 1\n+\n line 2\n",
			want: []protocol.Range{rng(1, 0, 0)},
		},
		{
			name: "insertion does not shift later removals",
			diff: "--- a\n+++ b\n@@ -1,3 +1,4 @@\n line 1\n+inserted\n line 2\n-line 3\n+LINE 3\n",
			want: []protocol.Range{rng(1, 0, 0), rng(2, 0, 6)},
		},
		{
			name: "hunk header without counts",
			diff: "--- a\n+++ b\n@@ -4 +4 @@\n-x\n+y\n",
			want: []protocol.Range{rng(3, 0, 1)},
		},
		{
			name: "multiple hunks",
			diff: "--- a\n+++ b\n@@ -1,3 +1,3 @@\n line 1\n-old 2\n+new 2\n line 3\n@@ -10,3 +10,3 @@\n line 10\n-old 11\n+new 11\n line 12\n",
			want: []protocol.Range{rng(1, 0, 5), rng(10, 0, 6)},
		},
		{
			name: "non-ASCII counted in UTF-16 units",
			diff: "--- a\n+++ b\n@@ -1 +1 @@\n-  $s = 'é😀';\n+  $s = \"é😀\";\n",
			// 2 spaces + `$s = '` (6) + é (1) + 😀 (2) + `';` (2) = 13
			want: []protocol.Range{rng(0, 2, 13)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dp.parseDiffForDiagnostics(tt.diff)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDiffForDiagnostics()\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}
