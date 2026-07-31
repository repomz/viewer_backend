package httpserver

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestXACachePreparesEveryFrameOnce(t *testing.T) {
	var rendered int
	orthanc := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		switch r.URL.Path {
		case "/dicom-web/studies/1.2.3/metadata":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"00080018": map[string]any{"Value": []any{"1.2.3.4"}},
					"0020000E": map[string]any{"Value": []any{"1.2.3.5"}},
					"00200011": map[string]any{"Value": []any{1}},
					"00280008": map[string]any{"Value": []any{3}},
				},
			})
		default:
			rendered++
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("jpeg-frame"))
		}
	}))
	defer orthanc.Close()

	root := t.TempDir()
	cache := &XACache{
		root:     root,
		pacsBase: orthanc.URL,
		client:   orthanc.Client(),
		queue:    make(chan string, 1),
		jobs:     make(map[string]*xaCacheJob),
	}
	cache.prepare("1.2.3")

	manifest, err := cache.readManifest("1.2.3")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest.FrameCount != 3 || len(manifest.Series) != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if rendered != 3 {
		t.Fatalf("rendered %d frames, expected 3", rendered)
	}
	for _, frame := range manifest.Series[0].Frames {
		if _, err := os.Stat(filepath.Join(root, "1.2.3", "frames", frame.ID)); err != nil {
			t.Fatalf("cached frame is missing: %v", err)
		}
	}
	if manifest.ArchivePath != "/xa-cache/1.2.3/archive" || manifest.ArchiveBytes <= 0 {
		t.Fatalf("archive is missing from manifest: %#v", manifest)
	}
	archive, err := zip.OpenReader(filepath.Join(root, "1.2.3", "frames.zip"))
	if err != nil {
		t.Fatalf("open XA archive: %v", err)
	}
	defer archive.Close()
	if len(archive.File) != 3 {
		t.Fatalf("archive contains %d frames, expected 3", len(archive.File))
	}

	// Existing static files are reused during a retry instead of being rendered again.
	cache.prepare("1.2.3")
	if rendered != 3 {
		t.Fatalf("retry rendered frames again: %d", rendered)
	}
}

func TestXACacheEnqueueIsIdempotent(t *testing.T) {
	cache := &XACache{
		root:   t.TempDir(),
		queue:  make(chan string, 4),
		jobs:   make(map[string]*xaCacheJob),
		client: &http.Client{Timeout: time.Second},
	}
	cache.Enqueue("1.2.840")
	cache.Enqueue("1.2.840")
	if len(cache.queue) != 1 {
		t.Fatalf("queue contains %d jobs, expected 1", len(cache.queue))
	}
	if status := cache.getStatus("1.2.840"); status.Status != "queued" {
		t.Fatalf("unexpected status: %#v", status)
	}
	if _, ok := <-cache.queue; !ok {
		t.Fatal("queue closed unexpectedly")
	}
}
