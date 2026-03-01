//go:build !cgo

package cmd

// scanAndDownloadGrammars detects needed languages and sets up grammars.
// Returns true when the user has pending steps — caller should halt init.
// The nocgo build has no GOMODCACHE fallback — it relies entirely on
// pre-built .so download via parsers.json.
func scanAndDownloadGrammars(root string) bool {
	handled, pending := grammarSetupFlow(root)
	if handled {
		return pending
	}

	// No parsers.json and no dev fallback — show manual download message.
	printParsersJSONMessage(root)
	return true
}
