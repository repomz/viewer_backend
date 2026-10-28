package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImagingDeletionRejectsUnsafeUIDs(t *testing.T) {
	cache := &XACache{root: t.TempDir()}
	for _, uid := range []string{"", ".", "..", "1..2", ".1", "1.", "../1", "1/2"} {
		if validStudyUID(uid) {
			t.Fatalf("unsafe UID accepted: %q", uid)
		}
		if err := cache.deleteStudy(context.Background(), uid); err == nil {
			t.Fatalf("unsafe deletion accepted: %q", uid)
		}
		if err := cache.saveDeletion(imagingDeletion{StudyUID: uid}); err == nil {
			t.Fatalf("unsafe job accepted: %q", uid)
		}
	}
}

func TestImagingDeletionRequiresAuthentication(t *testing.T) {
	h := HttpServer{}
	w := httptest.NewRecorder()
	h.RequireAdmin(h.RequestImagingDeletion)(w, httptest.NewRequest(http.MethodDelete, "/imaging-studies/1.2.3", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestImagingStorageFailurePreservesLocalFilesAndRecordsRetry(t *testing.T) {
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer storage.Close()
	cache := &XACache{root: t.TempDir(), cloud: &yandexArchive{endpoint: storage.URL, bucket: "bucket", client: storage.Client()}}
	if err := os.Mkdir(filepath.Join(cache.root, "1.2.3"), 0700); err != nil {
		t.Fatal(err)
	}
	h := HttpServer{xaCache: cache}
	h.runImagingDeletion(context.Background(), imagingDeletion{StudyUID: "1.2.3", Status: "pending"})
	jobs, err := cache.deletions()
	if err != nil || len(jobs) != 1 || jobs[0].Status != "failed" {
		t.Fatalf("jobs=%v err=%v", jobs, err)
	}
	if _, err := os.Stat(filepath.Join(cache.root, "1.2.3")); err != nil {
		t.Fatal("cache removed despite storage failure", err)
	}
}

func TestImagingObjectMatchesExactUID(t *testing.T) {
	for key, want := range map[string]bool{
		"viewer-xa/1.2.3/frames/a.jpg":            true,
		"Novikov_17.10.2026_1.2.3_attempt/a.dcm":  true,
		"Novikov_17.10.2026_1.2.3/a.dcm":          true,
		"viewer-xa/1.2.30/frames/a.jpg":           false,
		"Novikov_17.10.2026_1.2.30_attempt/a.dcm": false,
		"drive/user/Novikov_1.2.3/file":           false,
		"Novikov_1.2.3":                           false,
	} {
		if got := imagingObjectMatches(key, "1.2.3"); got != want {
			t.Errorf("%s: got %v", key, got)
		}
	}
}

func TestDeleteImagingVersionsAndMarkersWithPagination(t *testing.T) {
	deleted := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("unsigned storage request")
		}
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path+"?"+r.URL.RawQuery)
			w.WriteHeader(204)
			return
		}
		if r.URL.Query().Get("key-marker") == "" {
			_, _ = w.Write([]byte(`<ListVersionsResult><IsTruncated>true</IsTruncated><NextKeyMarker>next</NextKeyMarker><NextVersionIdMarker>v2</NextVersionIdMarker><Version><Key>viewer-xa/1.2.3/cines/a.mp4</Key><VersionId>v1</VersionId></Version><Version><Key>viewer-xa/1.2.30/cines/a.mp4</Key><VersionId>v1</VersionId></Version></ListVersionsResult>`))
		} else {
			if r.URL.Query().Get("version-id-marker") != "v2" {
				t.Error("missing version cursor")
			}
			_, _ = w.Write([]byte(`<ListVersionsResult><IsTruncated>false</IsTruncated><DeleteMarker><Key>P_1.2.3_attempt/a.dcm</Key><VersionId>v2</VersionId></DeleteMarker></ListVersionsResult>`))
		}
	}))
	defer server.Close()
	storage := &yandexArchive{endpoint: server.URL, bucket: "bucket", region: "ru-central1", client: server.Client()}
	if err := storage.deleteImagingObjects(context.Background(), "1.2.3"); err != nil {
		t.Fatal(err)
	}
	want := []string{"/bucket/viewer-xa/1.2.3/cines/a.mp4?versionId=v1", "/bucket/P_1.2.3_attempt/a.dcm?versionId=v2"}
	if !reflect.DeepEqual(deleted, want) {
		t.Fatalf("deleted %v", deleted)
	}
}

func TestImagingTombstoneSurvivesRestartAndBlocksPreparation(t *testing.T) {
	cache := &XACache{root: t.TempDir()}
	if err := cache.saveDeletion(imagingDeletion{StudyUID: "1.2.3", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	restarted := &XACache{root: cache.root}
	if !restarted.deletionRequested("1.2.3") {
		t.Fatal("lost tombstone")
	}
	restarted.Enqueue("1.2.3") // Must not touch nil queue/jobs or fetch PACS.
	restarted.prepare("1.2.3")
	if _, err := restarted.readManifest("1.2.3"); err == nil {
		t.Fatal("deleted manifest served")
	}
	jobs, err := restarted.deletions()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs=%v err=%v", jobs, err)
	}
}
