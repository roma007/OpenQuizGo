package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveImage(t *testing.T) {
	dir := t.TempDir()
	svc := New(dir, 5)
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	name, contentType, err := svc.save(png)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if contentType != "image/png" || filepath.Ext(name) != ".png" {
		t.Fatalf("unexpected type/ext: %s %s", contentType, name)
	}
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != int64(len(png)) {
		t.Fatalf("size = %d, want %d", info.Size(), len(png))
	}
}

func TestRejectNonImage(t *testing.T) {
	svc := New(t.TempDir(), 5)
	if _, _, err := svc.save([]byte("not an image")); err == nil {
		t.Fatal("expected error for non-image")
	}
}

func TestSafeName(t *testing.T) {
	cases := map[string]bool{
		"abc.png":       true,
		"../etc/passwd": false,
		"":              false,
		"a/b.png":       false,
	}
	for name, want := range cases {
		if got := safeName(name); got != want {
			t.Fatalf("safeName(%q) = %v, want %v", name, got, want)
		}
	}
}
