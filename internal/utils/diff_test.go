package utils_test

import (
	"testing"

	"github.com/cristianradulescu/php-diagls/internal/utils"
)

func TestApplyUnifiedDiff(t *testing.T) {
	tests := []struct {
		name     string
		original string
		diff     string
		expected string
		wantErr  bool
	}{
		{
			name: "simple single line change",
			original: `line 1
line 2
line 3`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
 line 1
-line 2
+line TWO
 line 3`,
			expected: `line 1
line TWO
line 3`,
			wantErr: false,
		},
		{
			name: "multiple changes in one hunk",
			original: `<?php
function hello() {
    echo "world";
}
?>`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,5 +1,5 @@
 <?php
-function hello() {
-    echo "world";
+function hello()
+{
+    echo 'world';
 }
 ?>`,
			expected: `<?php
function hello()
{
    echo 'world';
}
?>`,
			wantErr: false,
		},
		{
			name: "adding lines at beginning",
			original: `line 2
line 3`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,3 @@
+line 1
 line 2
 line 3`,
			expected: `line 1
line 2
line 3`,
			wantErr: false,
		},
		{
			name: "adding lines at end",
			original: `line 1
line 2`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,3 @@
 line 1
 line 2
+line 3`,
			expected: `line 1
line 2
line 3`,
			wantErr: false,
		},
		{
			name: "removing lines",
			original: `line 1
line 2
line 3
line 4`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,4 +1,2 @@
 line 1
-line 2
-line 3
 line 4`,
			expected: `line 1
line 4`,
			wantErr: false,
		},
		{
			name:     "empty diff returns original",
			original: "original content",
			diff:     "",
			expected: "original content",
			wantErr:  false,
		},
		{
			name:     "whitespace-only diff returns original",
			original: "original content",
			diff:     "   \n\n  \t\n",
			expected: "original content",
			wantErr:  false,
		},
		{
			name: "complex multi-hunk diff",
			original: `line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
 line 1
-line 2
+line TWO
 line 3
@@ -5,4 +5,4 @@
 line 5
-line 6
+line SIX
 line 7
 line 8`,
			expected: `line 1
line TWO
line 3
line 4
line 5
line SIX
line 7
line 8`,
			wantErr: false,
		},
		{
			name: "php-cs-fixer style diff - array formatting",
			original: `<?php
$array = array(1, 2, 3);`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,6 @@
 <?php
-$array = array(1, 2, 3);
+$array = [
+    1,
+    2,
+    3,
+];`,
			expected: `<?php
$array = [
    1,
    2,
    3,
];`,
			wantErr: false,
		},
		{
			name: "php-cs-fixer style diff - whitespace changes",
			original: `<?php
if($test){
echo "hello";
}`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,4 +1,4 @@
 <?php
-if($test){
-echo "hello";
+if ($test) {
+    echo 'hello';
 }`,
			expected: `<?php
if ($test) {
    echo 'hello';
}`,
			wantErr: false,
		},
		{
			name: "diff with context lines",
			original: `line 1
line 2
line 3
line 4
line 5`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,5 +1,5 @@
 line 1
 line 2
-line 3
+line THREE
 line 4
 line 5`,
			expected: `line 1
line 2
line THREE
line 4
line 5`,
			wantErr: false,
		},
		{
			name:     "entire file replacement",
			original: `old line 1\nold line 2`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,2 @@
-old line 1
-old line 2
+new line 1
+new line 2`,
			expected: `new line 1
new line 2`,
			wantErr: false,
		},
		{
			name: "diff with trailing newline handling",
			original: `line 1
line 2
`,
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,2 @@
-line 1
+line ONE
 line 2`,
			expected: `line ONE
line 2
`,
			wantErr: false,
		},
		{
			name:     "empty original content",
			original: "",
			diff: `--- a/test.php
+++ b/test.php
@@ -0,0 +1,2 @@
+new line 1
+new line 2`,
			// Note: ApplyUnifiedDiff adds a trailing newline when building result
			expected: `new line 1
new line 2
`,
			wantErr: false,
		},
		{
			name:     "malformed hunk header - should handle gracefully",
			original: "line 1\nline 2",
			diff: `--- a/test.php
+++ b/test.php
@@ invalid @@
 line 1
 line 2`,
			expected: "line 1\nline 2",
			wantErr:  false,
		},
		{
			name:     "diff with only header lines",
			original: "line 1\nline 2",
			diff: `--- a/test.php
+++ b/test.php`,
			expected: "line 1\nline 2",
			wantErr:  false,
		},
		{
			name:     "very long lines",
			original: "short\n" + string(make([]byte, 10000)) + "\nshort",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
-short
+modified
 ` + string(make([]byte, 10000)) + `
 short`,
			expected: "modified\n" + string(make([]byte, 10000)) + "\nshort",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.ApplyUnifiedDiff(tt.original, tt.diff)

			// Check error expectation
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Check result
			if result != tt.expected {
				t.Errorf("\n=== ApplyUnifiedDiff() Result Mismatch ===\n"+
					"Got:\n%q\n\n"+
					"Want:\n%q\n"+
					"=====================================",
					result, tt.expected)
			}
		})
	}
}

// TestApplyUnifiedDiff_EdgeCases tests specific edge cases that need special attention
func TestApplyUnifiedDiff_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		original string
		diff     string
		expected string
	}{
		{
			name:     "diff with no changes (only context)",
			original: "line 1\nline 2\nline 3",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,3 +1,3 @@
 line 1
 line 2
 line 3`,
			expected: "line 1\nline 2\nline 3",
		},
		{
			name:     "adding line in middle of file",
			original: "line 1\nline 3",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,3 @@
 line 1
+line 2
 line 3`,
			expected: "line 1\nline 2\nline 3",
		},
		{
			name:     "multiple consecutive additions",
			original: "line 1\nline 5",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,2 +1,5 @@
 line 1
+line 2
+line 3
+line 4
 line 5`,
			expected: "line 1\nline 2\nline 3\nline 4\nline 5",
		},
		{
			name:     "multiple consecutive deletions",
			original: "line 1\nline 2\nline 3\nline 4\nline 5",
			diff: `--- a/test.php
+++ b/test.php
@@ -1,5 +1,2 @@
 line 1
-line 2
-line 3
-line 4
 line 5`,
			expected: "line 1\nline 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.ApplyUnifiedDiff(tt.original, tt.diff)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("\nGot:\n%q\n\nWant:\n%q", result, tt.expected)
			}
		})
	}
}

// TestApplyUnifiedDiff_RealWorldExamples tests with actual php-cs-fixer output examples
func TestApplyUnifiedDiff_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name     string
		original string
		diff     string
		expected string
	}{
		{
			name: "real php-cs-fixer - single_quote rule",
			original: `<?php
echo "Hello World";`,
			diff: `--- a.php
+++ b.php
@@ -1,2 +1,2 @@
 <?php
-echo "Hello World";
+echo 'Hello World';`,
			expected: `<?php
echo 'Hello World';`,
		},
		{
			name: "real php-cs-fixer - braces rule",
			original: `<?php
function test(){
return true;
}`,
			diff: `--- a.php
+++ b.php
@@ -1,4 +1,5 @@
 <?php
-function test(){
-return true;
+function test()
+{
+    return true;
 }`,
			expected: `<?php
function test()
{
    return true;
}`,
		},
		{
			name: "real php-cs-fixer - no_unused_imports rule",
			original: `<?php
use Vendor\Package\ClassA;
use Vendor\Package\ClassB;

$a = new ClassA();`,
			diff: `--- a.php
+++ b.php
@@ -1,5 +1,4 @@
 <?php
 use Vendor\Package\ClassA;
-use Vendor\Package\ClassB;
 
 $a = new ClassA();`,
			expected: `<?php
use Vendor\Package\ClassA;

$a = new ClassA();`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := utils.ApplyUnifiedDiff(tt.original, tt.diff)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("\nGot:\n%s\n\nWant:\n%s", result, tt.expected)
			}
		})
	}
}
