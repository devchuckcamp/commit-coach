package testutil

import (
	"strings"

	"github.com/chuckie/commit-coach/internal/ports"
)

// SampleDiffSmall is a small sample diff for testing.
const SampleDiffSmall = `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,5 +1,10 @@
 package main
 
+import "fmt"
+
 func main() {
-    println("Hello")
+    fmt.Println("Hello, World!")
 }
`

// SampleDiffLarge is a large sample diff for testing diff capping (generated at runtime).
var SampleDiffLarge = func() string {
	const header = `diff --git a/very_long_file.go b/very_long_file.go
index 1234567..abcdefg 100644
--- a/very_long_file.go
+++ b/very_long_file.go
@@ -1,5 +1,1000 @@
 package main

`
	return header + strings.Repeat("// This is a very long comment line that repeats\n", 200)
}()

// SampleLLMResponse returns a sample valid LLM response.
func SampleLLMResponse() []ports.CommitSuggestion {
	return []ports.CommitSuggestion{
		{
			Type:    "feat",
			Subject: "add LLM provider abstraction",
			Body:    "Implement ports.LLM interface to support multiple providers like OpenAI and Groq.",
			Footer:  "",
		},
		{
			Type:    "fix",
			Subject: "handle empty staged diff",
			Body:    "",
			Footer:  "",
		},
		{
			Type:    "refactor",
			Subject: "split redaction into security package",
			Body:    "Move redaction logic into internal/security for better organization.",
			Footer:  "BREAKING CHANGE: redaction API moved",
		},
	}
}

// SampleInvalidSuggestion returns a suggestion with invalid data.
func SampleInvalidSuggestion() ports.CommitSuggestion {
	return ports.CommitSuggestion{
		Type:    "invalid_type",
		Subject: "This is a subject that is way too long and exceeds the 72 character limit by far",
		Body:    "",
		Footer:  "",
	}
}
