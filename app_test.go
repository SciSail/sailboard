package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilePathsMissing(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(present, []byte("clipboard file"), 0644); err != nil {
		t.Fatal(err)
	}
	if filePathsMissing(present) {
		t.Fatal("existing file reported as missing")
	}
	if !filePathsMissing(filepath.Join(dir, "gone.txt")) {
		t.Fatal("missing file reported as present")
	}
	if !filePathsMissing(present + "\n" + filepath.Join(dir, "gone.txt")) {
		t.Fatal("entry with one missing file reported as present")
	}
	if filePathsMissing(present + "\n" + present) {
		t.Fatal("entry with all files present reported as missing")
	}
	if !filePathsMissing("") {
		t.Fatal("empty file path reported as present")
	}
}
