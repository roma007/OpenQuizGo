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

func TestSaveAudio(t *testing.T) {
	cases := []struct {
		name        string
		data        []byte
		ext         string
		contentType string
	}{
		{"mp3-id3", []byte("ID3\x04\x00\x00\x00\x00\x00\x00"), ".mp3", "audio/mpeg"},
		{"mp3-frame", []byte{0xFF, 0xFB, 0x90, 0x64, 0x00, 0x00}, ".mp3", "audio/mpeg"},
		{"wav", []byte("RIFF\x24\x00\x00\x00WAVEfmt "), ".wav", "audio/wav"},
		{"ogg", []byte("OggS\x00\x02\x00\x00"), ".ogg", "audio/ogg"},
		{"m4a", []byte("\x00\x00\x00\x20ftypM4A \x00\x00\x00\x00"), ".m4a", "audio/mp4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := New(t.TempDir(), 5)
			name, contentType, err := svc.save(tc.data)
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			if contentType != tc.contentType || filepath.Ext(name) != tc.ext {
				t.Fatalf("unexpected type/ext: %s %s", contentType, name)
			}
		})
	}
}

func TestRejectUnknownMedia(t *testing.T) {
	svc := New(t.TempDir(), 5)
	if _, _, err := svc.save([]byte("plain text")); err == nil {
		t.Fatal("expected error for unknown media")
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
