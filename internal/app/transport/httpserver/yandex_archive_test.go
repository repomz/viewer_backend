package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestYandexArchiveUploadsObjectsBeforeManifestAndVerifiesSizes(t *testing.T) {
	var mu sync.Mutex
	objects := map[string][]byte{}
	putOrder := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("signed request has no authorization header")
		}
		switch r.Method {
		case http.MethodPut:
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			objects[r.URL.Path] = payload
			putOrder = append(putOrder, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodHead:
			mu.Lock()
			payload, ok := objects[r.URL.Path]
			mu.Unlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	studyUID := "1.2.840.1"
	if err := os.MkdirAll(filepath.Join(root, studyUID, "cines"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, studyUID, "frames"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, studyUID, "cines", "series.mp4"), []byte("cine"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, studyUID, "frames", "frame.jpg"), []byte("frame"), 0o640); err != nil {
		t.Fatal(err)
	}
	archive := &yandexArchive{
		endpoint: server.URL, bucket: "bucket", accessKey: "access",
		secretKey: "secret", region: "ru-central1", client: server.Client(),
	}
	manifest := xaCacheManifest{
		Status: "ready", StudyUID: studyUID, FrameCount: 1,
		Series: []xaCacheSeries{{
			CineID: "series.mp4",
			Frames: []xaCacheFrame{{ID: "frame.jpg"}},
		}},
	}
	if err := archive.uploadStudy(context.Background(), root, manifest); err != nil {
		t.Fatal(err)
	}
	if len(putOrder) != 3 || !strings.HasSuffix(putOrder[len(putOrder)-1], "/manifest.json") {
		t.Fatalf("manifest must be committed last, PUT order: %v", putOrder)
	}
}
