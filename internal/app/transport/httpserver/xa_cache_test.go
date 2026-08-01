package httpserver

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

func TestXACacheBuildsFastStartCinePerSeries(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is not installed")
	}
	root := t.TempDir()
	studyUID := "1.2.3"
	framesDirectory := filepath.Join(root, studyUID, "frames")
	if err := os.MkdirAll(framesDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	frames := make([]xaCacheFrame, 0, 3)
	for index := range 3 {
		id := fmt.Sprintf("frame-%d.jpg", index+1)
		file, createErr := os.Create(filepath.Join(framesDirectory, id))
		if createErr != nil {
			t.Fatal(createErr)
		}
		picture := image.NewGray(image.Rect(0, 0, 64, 64))
		for pixel := range picture.Pix {
			picture.Pix[pixel] = color.Gray{Y: uint8(index * 60)}.Y
		}
		if encodeErr := jpeg.Encode(file, picture, &jpeg.Options{Quality: 85}); encodeErr != nil {
			_ = file.Close()
			t.Fatal(encodeErr)
		}
		_ = file.Close()
		frames = append(frames, xaCacheFrame{ID: id})
	}
	cache := &XACache{root: root, ffmpegPath: ffmpeg}
	manifest := xaCacheManifest{
		StudyUID: studyUID,
		Series: []xaCacheSeries{{
			SeriesUID: "1.2.3.4",
			FPS:       12,
			Frames:    frames,
		}},
	}
	if err := cache.writeCines(&manifest); err != nil {
		t.Fatalf("write cines: %v", err)
	}
	series := manifest.Series[0]
	if series.CinePath == "" || series.CineBytes <= 0 {
		t.Fatalf("cine is missing from manifest: %#v", series)
	}
	content, err := os.ReadFile(cache.cinePath(studyUID, series.CineID))
	if err != nil {
		t.Fatal(err)
	}
	moov := bytes.Index(content, []byte("moov"))
	mdat := bytes.Index(content, []byte("mdat"))
	if moov < 0 || mdat < 0 || moov > mdat {
		t.Fatalf("MP4 is not faststart: moov=%d mdat=%d", moov, mdat)
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
