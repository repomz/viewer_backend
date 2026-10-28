package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/repomz/viewer_backend/internal/app/common/server"
)

// Serializes mutations of one imaging study, never unrelated studies.
var imagingLocks sync.Map

func imagingLock(uid string) *sync.Mutex {
	value, _ := imagingLocks.LoadOrStore(uid, &sync.Mutex{})
	return value.(*sync.Mutex)
}

type imagingDeletion struct {
	StudyUID  string    `json:"study_uid"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     string    `json:"error,omitempty"`
}

func (c *XACache) deletionPath(uid string) string {
	return filepath.Join(c.root, ".deletions", uid+".json")
}
func (c *XACache) deletionRequested(uid string) bool {
	if !validStudyUID(uid) {
		return false
	}
	_, err := os.Stat(c.deletionPath(uid))
	return err == nil
}
func (c *XACache) saveDeletion(job imagingDeletion) error {
	if !validStudyUID(job.StudyUID) {
		return fmt.Errorf("invalid StudyInstanceUID")
	}
	dir := filepath.Join(c.root, ".deletions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	job.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".job-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), c.deletionPath(job.StudyUID))
}
func (c *XACache) deletions() ([]imagingDeletion, error) {
	entries, err := os.ReadDir(filepath.Join(c.root, ".deletions"))
	if os.IsNotExist(err) {
		return []imagingDeletion{}, nil
	}
	if err != nil {
		return nil, err
	}
	jobs := []imagingDeletion{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(c.root, ".deletions", entry.Name()))
		if err != nil {
			return nil, err
		}
		var job imagingDeletion
		if err := json.Unmarshal(data, &job); err != nil {
			return nil, err
		}
		if validStudyUID(job.StudyUID) {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}
func (h HttpServer) ListImagingDeletions(w http.ResponseWriter, r *http.Request) {
	if h.xaCache == nil {
		server.RespondOK([]imagingDeletion{}, w, r)
		return
	}
	jobs, err := h.xaCache.deletions()
	if err != nil {
		http.Error(w, "Cannot read deletion jobs", 500)
		return
	}
	server.RespondOK(jobs, w, r)
}
func (h HttpServer) RequestImagingDeletion(w http.ResponseWriter, r *http.Request) {
	uid := mux.Vars(r)["study_uid"]
	if !validStudyUID(uid) {
		http.Error(w, "Invalid study UID", 400)
		return
	}
	if h.xaCache == nil || h.xaCache.cloud == nil || h.sqlDB == nil {
		http.Error(w, "Imaging storage is not configured", 503)
		return
	}
	var exists bool
	if err := h.sqlDB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM studies WHERE study_id=$1 AND lower(study_type) IN ('xa','ct'))`, uid).Scan(&exists); err != nil {
		http.Error(w, "Cannot resolve imaging study", 500)
		return
	}
	if !exists {
		http.Error(w, "Imaging study not found", 404)
		return
	}
	// A persistent tombstone blocks preparation and re-import even after restart.
	job := imagingDeletion{StudyUID: uid, Status: "pending"}
	if err := h.xaCache.saveDeletion(job); err != nil {
		http.Error(w, "Cannot queue deletion", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(job)
}

// Resume interrupted work after a restart. One deletion at a time avoids
// saturating Orthanc and S3. Failed jobs remain visible and are retried.
func (h HttpServer) StartImagingDeletions(ctx context.Context) {
	if h.xaCache == nil || h.sqlDB == nil {
		return
	}
	for {
		jobs, err := h.xaCache.deletions()
		if err == nil {
			for _, job := range jobs {
				if job.Status == "completed" {
					continue
				}
				if job.Status == "failed" && time.Since(job.UpdatedAt) < 5*time.Minute {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				h.runImagingDeletion(ctx, job)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}
func (h HttpServer) runImagingDeletion(parent context.Context, job imagingDeletion) {
	lock := imagingLock(job.StudyUID)
	lock.Lock()
	defer lock.Unlock()
	ctx, cancel := context.WithTimeout(parent, 30*time.Minute)
	defer cancel()
	job.Status = "deleting"
	job.Error = ""
	_ = h.xaCache.saveDeletion(job)
	err := h.xaCache.cloud.deleteImagingObjects(ctx, job.StudyUID)
	if err == nil {
		err = h.xaCache.deleteStudy(ctx, job.StudyUID)
	}
	if err == nil {
		_, err = h.sqlDB.ExecContext(ctx, `UPDATE studies SET deleted=true,updated_at=NOW() WHERE study_id=$1 AND lower(study_type) IN ('xa','ct')`, job.StudyUID)
	}
	if err == nil {
		_, err = h.sqlDB.ExecContext(ctx, `UPDATE studies SET dicom_link='',updated_at=NOW() WHERE dicom_link=$1`, "xa://study/"+job.StudyUID)
	}
	if err != nil {
		job.Status = "failed"
		log.Printf("Imaging deletion failed: study_uid=%s error=%v", job.StudyUID, err)
		// Do not expose signed URLs or storage credentials through the status API.
		job.Error = "Не удалось завершить очистку хранилищ. Повторная попытка будет выполнена автоматически."
	} else {
		job.Status = "completed"
		invalidateStudyAnalysisResponseCaches()
	}
	_ = h.xaCache.saveDeletion(job)
}

func imagingObjectMatches(key, uid string) bool {
	if strings.HasPrefix(key, "viewer-xa/"+uid+"/") {
		return true
	}
	folder, _, found := strings.Cut(key, "/")
	if !found {
		return false
	}
	// Agent folders: Patient_date_StudyInstanceUID[_attempt]. Never substring-match a UID.
	return strings.HasSuffix(folder, "_"+uid) || strings.Contains(folder, "_"+uid+"_")
}
func (s *yandexArchive) queryRequest(ctx context.Context, method, key string, query url.Values) (*http.Response, error) {
	target := s.endpoint + "/" + url.PathEscape(s.bucket) + "/" + escapeObjectKey(key) + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(nil)
	return s.signAndDo(req, "", hex.EncodeToString(digest[:]))
}
func (s *yandexArchive) deleteImagingObjects(ctx context.Context, uid string) error {
	if !validStudyUID(uid) {
		return fmt.Errorf("invalid StudyInstanceUID")
	}
	if s == nil {
		return fmt.Errorf("cloud storage unavailable")
	}
	// Enumerate versions, including delete markers: deleting just current objects
	// in a versioned bucket would leave the actual DICOM files behind.
	type objectVersion struct {
		Key       string
		VersionID string `xml:"VersionId"`
	}
	query := url.Values{"versions": {""}, "max-keys": {"1000"}}
	for {
		response, err := s.queryRequest(ctx, http.MethodGet, "", query)
		if err != nil {
			return err
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			return fmt.Errorf("list storage versions: HTTP %d", response.StatusCode)
		}
		var page struct {
			Versions    []objectVersion `xml:"Version"`
			Markers     []objectVersion `xml:"DeleteMarker"`
			More        bool            `xml:"IsTruncated"`
			NextKey     string          `xml:"NextKeyMarker"`
			NextVersion string          `xml:"NextVersionIdMarker"`
		}
		err = xml.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if err != nil {
			return err
		}
		for _, object := range append(page.Versions, page.Markers...) {
			if !imagingObjectMatches(object.Key, uid) {
				continue
			}
			result, err := s.queryRequest(ctx, http.MethodDelete, object.Key, url.Values{"versionId": {object.VersionID}})
			if err != nil {
				return err
			}
			result.Body.Close()
			if result.StatusCode != 204 && result.StatusCode != 200 && result.StatusCode != 404 {
				return fmt.Errorf("delete storage version: HTTP %d", result.StatusCode)
			}
		}
		if !page.More {
			return nil
		}
		if page.NextKey == "" || (query.Get("key-marker") == page.NextKey && query.Get("version-id-marker") == page.NextVersion) {
			return fmt.Errorf("invalid storage pagination")
		}
		query.Set("key-marker", page.NextKey)
		query.Set("version-id-marker", page.NextVersion)
	}
}
