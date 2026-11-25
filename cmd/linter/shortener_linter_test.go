package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestShortenerLinter(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), ShortenerLinter, "a", "mainpkg")
}
