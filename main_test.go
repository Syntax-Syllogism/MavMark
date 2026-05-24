package main

import (
	"path/filepath"
	"testing"
)

func TestTaskPathAcceptsFlatFilenames(t *testing.T) {
	path, err := taskPath("docs", "notes..draft.md")
	if err != nil {
		t.Fatalf("taskPath returned error: %v", err)
	}

	want := filepath.Join("docs", "notes..draft.md")
	if path != want {
		t.Fatalf("taskPath() = %q, want %q", path, want)
	}
}

func TestTaskPathRejectsMissingFile(t *testing.T) {
	if _, err := taskPath("docs", ""); err == nil {
		t.Fatal("taskPath accepted empty file")
	}
}

func TestTaskPathRejectsNestedAndTraversalPaths(t *testing.T) {
	tests := []string{
		"../main.go",
		"foo/../bar.md",
		"subdir/card.md",
		filepath.Join("subdir", "card.md"),
		filepath.Join("..", "main.go"),
	}

	for _, file := range tests {
		t.Run(file, func(t *testing.T) {
			if _, err := taskPath("docs", file); err == nil {
				t.Fatalf("taskPath accepted %q", file)
			}
		})
	}
}

func TestTaskPathRejectsAbsolutePaths(t *testing.T) {
	if _, err := taskPath("docs", filepath.Join(string(filepath.Separator), "tmp", "card.md")); err == nil {
		t.Fatal("taskPath accepted absolute path")
	}
}
