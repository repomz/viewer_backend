package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"github.com/repomz/viewer_backend/internal/app/common/server"
)

const (
	xaCacheViewport = "768,768"
	xaCacheQuality  = "85"
)

type dicomJSONValue struct {
	Value []any `json:"Value"`
}

type xaCacheFrame struct {
	ID          string `json:"id"`
	InstanceUID string `json:"instance_uid"`
	FrameIndex  int    `json:"frame_index"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
}

type xaCacheSeries struct {
	SeriesUID   string         `json:"series_uid"`
	Number      int            `json:"number"`
	Description string         `json:"description,omitempty"`
	Frames      []xaCacheFrame `json:"frames"`
}

type xaCacheManifest struct {
	Status     string          `json:"status"`
	StudyUID   string          `json:"study_uid"`
	Prepared   time.Time       `json:"prepared_at"`
	FrameCount int             `json:"frame_count"`
	TotalBytes int64           `json:"total_bytes"`
	Series     []xaCacheSeries `json:"series"`
}

type xaCacheStatus struct {
	Status     string `json:"status"`
	StudyUID   string `json:"study_uid"`
	FrameCount int    `json:"frame_count,omitempty"`
	Prepared   int    `json:"prepared_frames,omitempty"`
	TotalBytes int64  `json:"total_bytes,omitempty"`
	Error      string `json:"error,omitempty"`
}

type xaCacheJob struct {
	status xaCacheStatus
}

// XACache prepares Orthanc rendered frames once and serves them as immutable files.
type XACache struct {
	root       string
	pacsBase   string
	username   string
	password   string
	client     *http.Client
	queue      chan string
	jobs       map[string]*xaCacheJob
	jobsMu     sync.RWMutex
	workerOnce sync.Once
}

func NewXACacheFromEnvironment() (*XACache, error) {
	root := strings.TrimSpace(os.Getenv("XA_CACHE_DIR"))
	if root == "" {
		root = "/app/xa-cache"
	}
	remoteURL := strings.TrimRight(strings.TrimSpace(os.Getenv("REMOTE_PACS_URL")), "/")
	if remoteURL == "" {
		return nil, errors.New("REMOTE_PACS_URL is not configured")
	}
	pacsBase := strings.TrimSuffix(remoteURL, "/instances")
	timeoutSeconds, err := strconv.Atoi(os.Getenv("XA_CACHE_TIMEOUT_SECONDS"))
	if err != nil || timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create XA cache directory: %w", err)
	}
	cache := &XACache{
		root:     root,
		pacsBase: pacsBase,
		username: os.Getenv("REMOTE_PACS_USERNAME"),
		password: os.Getenv("REMOTE_PACS_PASSWORD"),
		client:   &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		queue:    make(chan string, 128),
		jobs:     make(map[string]*xaCacheJob),
	}
	cache.start()
	return cache, nil
}

func (c *XACache) start() {
	c.workerOnce.Do(func() {
		go func() {
			for studyUID := range c.queue {
				c.prepare(studyUID)
			}
		}()
	})
}

func validStudyUID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && character != '.' {
			return false
		}
	}
	return true
}

func (c *XACache) manifestPath(studyUID string) string {
	return filepath.Join(c.root, studyUID, "manifest.json")
}

func (c *XACache) Enqueue(studyUID string) {
	if !validStudyUID(studyUID) {
		return
	}
	if _, err := os.Stat(c.manifestPath(studyUID)); err == nil {
		c.setStatus(xaCacheStatus{Status: "ready", StudyUID: studyUID})
		return
	}
	c.jobsMu.Lock()
	if current, ok := c.jobs[studyUID]; ok &&
		(current.status.Status == "queued" || current.status.Status == "preparing") {
		c.jobsMu.Unlock()
		return
	}
	c.jobs[studyUID] = &xaCacheJob{status: xaCacheStatus{
		Status:   "queued",
		StudyUID: studyUID,
	}}
	c.jobsMu.Unlock()
	select {
	case c.queue <- studyUID:
	default:
		go func() { c.queue <- studyUID }()
	}
}

func (c *XACache) setStatus(status xaCacheStatus) {
	c.jobsMu.Lock()
	c.jobs[status.StudyUID] = &xaCacheJob{status: status}
	c.jobsMu.Unlock()
}

func (c *XACache) getStatus(studyUID string) xaCacheStatus {
	if manifest, err := c.readManifest(studyUID); err == nil {
		return xaCacheStatus{
			Status:     "ready",
			StudyUID:   studyUID,
			FrameCount: manifest.FrameCount,
			Prepared:   manifest.FrameCount,
			TotalBytes: manifest.TotalBytes,
		}
	}
	c.jobsMu.RLock()
	defer c.jobsMu.RUnlock()
	if job, ok := c.jobs[studyUID]; ok {
		return job.status
	}
	return xaCacheStatus{Status: "missing", StudyUID: studyUID}
}

func tagString(item map[string]dicomJSONValue, tag string) string {
	values := item[tag].Value
	if len(values) == 0 {
		return ""
	}
	value, _ := values[0].(string)
	return value
}

func tagInt(item map[string]dicomJSONValue, tag string, fallback int) int {
	values := item[tag].Value
	if len(values) == 0 {
		return fallback
	}
	switch value := values[0].(type) {
	case float64:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func (c *XACache) request(ctx context.Context, method, endpoint string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.pacsBase+endpoint, nil)
	if err != nil {
		return nil, err
	}
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	return c.client.Do(request)
}

func (c *XACache) loadMetadata(
	ctx context.Context,
	studyUID string,
) ([]map[string]dicomJSONValue, error) {
	response, err := c.request(
		ctx,
		http.MethodGet,
		"/dicom-web/studies/"+studyUID+"/metadata",
	)
	if err != nil {
		return nil, fmt.Errorf("request DICOM metadata: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("request DICOM metadata: HTTP %d", response.StatusCode)
	}
	var metadata []map[string]dicomJSONValue
	if err := json.NewDecoder(response.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode DICOM metadata: %w", err)
	}
	return metadata, nil
}

type xaFrameSource struct {
	seriesIndex int
	frameIndex  int
	instanceUID string
	id          string
	url         string
	path        string
}

func (c *XACache) buildManifest(
	studyUID string,
	metadata []map[string]dicomJSONValue,
) (xaCacheManifest, []xaFrameSource, error) {
	manifest := xaCacheManifest{
		Status:   "ready",
		StudyUID: studyUID,
		Series:   make([]xaCacheSeries, 0),
	}
	seriesIndexes := make(map[string]int)
	sources := make([]xaFrameSource, 0)
	for _, instance := range metadata {
		instanceUID := tagString(instance, "00080018")
		seriesUID := tagString(instance, "0020000E")
		if instanceUID == "" || seriesUID == "" {
			continue
		}
		seriesIndex, exists := seriesIndexes[seriesUID]
		if !exists {
			seriesIndex = len(manifest.Series)
			seriesIndexes[seriesUID] = seriesIndex
			manifest.Series = append(manifest.Series, xaCacheSeries{
				SeriesUID:   seriesUID,
				Number:      tagInt(instance, "00200011", seriesIndex+1),
				Description: tagString(instance, "0008103E"),
				Frames:      make([]xaCacheFrame, 0),
			})
		}
		frameCount := max(1, tagInt(instance, "00280008", 1))
		for frameIndex := 1; frameIndex <= frameCount; frameIndex++ {
			key := fmt.Sprintf("%s/%d", instanceUID, frameIndex)
			digest := sha256.Sum256([]byte(key))
			id := hex.EncodeToString(digest[:12]) + ".jpg"
			framePath := filepath.Join(c.root, studyUID, "frames", id)
			source := xaFrameSource{
				seriesIndex: seriesIndex,
				frameIndex:  frameIndex,
				instanceUID: instanceUID,
				id:          id,
				url: fmt.Sprintf(
					"%s/dicom-web/studies/%s/series/%s/instances/%s/frames/%d/rendered?viewport=%s&quality=%s",
					c.pacsBase,
					studyUID,
					seriesUID,
					instanceUID,
					frameIndex,
					xaCacheViewport,
					xaCacheQuality,
				),
				path: framePath,
			}
			sources = append(sources, source)
			manifest.Series[seriesIndex].Frames = append(
				manifest.Series[seriesIndex].Frames,
				xaCacheFrame{
					ID:          id,
					InstanceUID: instanceUID,
					FrameIndex:  frameIndex,
					Path:        "/xa-cache/" + studyUID + "/frames/" + id,
				},
			)
		}
	}
	if len(sources) == 0 {
		return xaCacheManifest{}, nil, errors.New("DICOM study contains no renderable frames")
	}
	return manifest, sources, nil
}

func (c *XACache) prepare(studyUID string) {
	status := xaCacheStatus{Status: "preparing", StudyUID: studyUID}
	c.setStatus(status)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	metadata, err := c.loadMetadata(ctx, studyUID)
	if err != nil {
		c.fail(studyUID, err)
		return
	}
	manifest, sources, err := c.buildManifest(studyUID, metadata)
	if err != nil {
		c.fail(studyUID, err)
		return
	}
	status.FrameCount = len(sources)
	c.setStatus(status)
	if err := os.MkdirAll(filepath.Join(c.root, studyUID, "frames"), 0o750); err != nil {
		c.fail(studyUID, err)
		return
	}

	type result struct {
		source xaFrameSource
		size   int64
		err    error
	}
	sourceQueue := make(chan xaFrameSource)
	results := make(chan result, len(sources))
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for source := range sourceQueue {
				size, downloadErr := c.downloadFrame(ctx, source)
				results <- result{source: source, size: size, err: downloadErr}
			}
		}()
	}
	go func() {
		for _, source := range sources {
			sourceQueue <- source
		}
		close(sourceQueue)
		workers.Wait()
		close(results)
	}()

	for prepared := range results {
		if prepared.err != nil {
			cancel()
			c.fail(studyUID, prepared.err)
			return
		}
		manifest.Series[prepared.source.seriesIndex].
			Frames[framePosition(manifest.Series[prepared.source.seriesIndex], prepared.source)].
			Size = prepared.size
		status.Prepared++
		status.TotalBytes += prepared.size
		c.setStatus(status)
	}
	manifest.Prepared = time.Now()
	manifest.FrameCount = status.Prepared
	manifest.TotalBytes = status.TotalBytes
	if err := c.writeManifest(manifest); err != nil {
		c.fail(studyUID, err)
		return
	}
	c.setStatus(xaCacheStatus{
		Status:     "ready",
		StudyUID:   studyUID,
		FrameCount: manifest.FrameCount,
		Prepared:   manifest.FrameCount,
		TotalBytes: manifest.TotalBytes,
	})
	log.Printf(
		"XA cine cache prepared: study_uid=%s frames=%d bytes=%d",
		studyUID,
		manifest.FrameCount,
		manifest.TotalBytes,
	)
}

func framePosition(series xaCacheSeries, source xaFrameSource) int {
	for index, frame := range series.Frames {
		if frame.InstanceUID == source.instanceUID && frame.FrameIndex == source.frameIndex {
			return index
		}
	}
	return 0
}

func (c *XACache) downloadFrame(ctx context.Context, source xaFrameSource) (int64, error) {
	if info, err := os.Stat(source.path); err == nil && info.Size() > 0 {
		return info.Size(), nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "image/jpeg")
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("render XA frame: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return 0, fmt.Errorf("render XA frame: HTTP %d", response.StatusCode)
	}
	temporary, err := os.CreateTemp(filepath.Dir(source.path), ".frame-*.tmp")
	if err != nil {
		return 0, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	size, copyErr := io.Copy(temporary, response.Body)
	closeErr := temporary.Close()
	if copyErr != nil {
		return 0, copyErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if size == 0 {
		return 0, errors.New("Orthanc returned an empty XA frame")
	}
	if err := os.Rename(temporaryPath, source.path); err != nil {
		return 0, err
	}
	return size, nil
}

func (c *XACache) fail(studyUID string, err error) {
	log.Printf("XA cine cache failed: study_uid=%s error=%v", studyUID, err)
	c.setStatus(xaCacheStatus{
		Status:   "error",
		StudyUID: studyUID,
		Error:    err.Error(),
	})
}

func (c *XACache) writeManifest(manifest xaCacheManifest) error {
	directory := filepath.Join(c.root, manifest.StudyUID)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".manifest-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoder := json.NewEncoder(temporary)
	if err := encoder.Encode(manifest); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, c.manifestPath(manifest.StudyUID))
}

func (c *XACache) readManifest(studyUID string) (xaCacheManifest, error) {
	file, err := os.Open(c.manifestPath(studyUID))
	if err != nil {
		return xaCacheManifest{}, err
	}
	defer file.Close()
	var manifest xaCacheManifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return xaCacheManifest{}, err
	}
	return manifest, nil
}

func (c *XACache) WarmExisting(ctx context.Context) {
	response, err := c.request(ctx, http.MethodGet, "/dicom-web/studies?includefield=00080061")
	if err != nil {
		log.Printf("XA cache warmup skipped: %v", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		log.Printf("XA cache warmup skipped: PACS HTTP %d", response.StatusCode)
		return
	}
	var studies []map[string]dicomJSONValue
	if err := json.NewDecoder(response.Body).Decode(&studies); err != nil {
		log.Printf("XA cache warmup skipped: %v", err)
		return
	}
	for _, study := range studies {
		modalities := study["00080061"].Value
		hasXA := false
		for _, modality := range modalities {
			if strings.EqualFold(fmt.Sprint(modality), "XA") {
				hasXA = true
				break
			}
		}
		if hasXA {
			c.Enqueue(tagString(study, "0020000D"))
		}
	}
}

func (h HttpServer) GetXACacheManifest(w http.ResponseWriter, r *http.Request) {
	if h.xaCache == nil {
		http.Error(w, "XA cache is disabled", http.StatusServiceUnavailable)
		return
	}
	studyUID := mux.Vars(r)["study_uid"]
	if !validStudyUID(studyUID) {
		server.BadRequest("invalid-study-uid", errors.New("invalid StudyInstanceUID"), w, r)
		return
	}
	manifest, err := h.xaCache.readManifest(studyUID)
	if err == nil {
		server.RespondOK(manifest, w, r)
		return
	}
	h.xaCache.Enqueue(studyUID)
	status := h.xaCache.getStatus(studyUID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "2")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(status)
}

func (h HttpServer) PrepareXACache(w http.ResponseWriter, r *http.Request) {
	if h.xaCache == nil {
		http.Error(w, "XA cache is disabled", http.StatusServiceUnavailable)
		return
	}
	studyUID := mux.Vars(r)["study_uid"]
	if !validStudyUID(studyUID) {
		server.BadRequest("invalid-study-uid", errors.New("invalid StudyInstanceUID"), w, r)
		return
	}
	h.xaCache.Enqueue(studyUID)
	server.RespondOK(h.xaCache.getStatus(studyUID), w, r)
}

func (h HttpServer) GetXACacheFrame(w http.ResponseWriter, r *http.Request) {
	if h.xaCache == nil {
		http.NotFound(w, r)
		return
	}
	studyUID := mux.Vars(r)["study_uid"]
	frameID := mux.Vars(r)["frame_id"]
	if !validStudyUID(studyUID) ||
		!strings.HasSuffix(frameID, ".jpg") ||
		strings.ContainsAny(frameID, `/\`) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.xaCache.root, studyUID, "frames", frameID)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, path)
}
