package httpserver

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type yandexArchive struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
	region    string
	client    *http.Client
}

func newYandexArchiveFromEnvironment() *yandexArchive {
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("YANDEX_ENDPOINT")), "/")
	bucket := strings.TrimSpace(os.Getenv("YANDEX_BUCKET"))
	accessKey := strings.TrimSpace(os.Getenv("YANDEX_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("YANDEX_SECRET_ACCESS_KEY"))
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil
	}
	region := strings.TrimSpace(os.Getenv("YANDEX_REGION"))
	if region == "" {
		region = "ru-central1"
	}
	return &yandexArchive{
		endpoint: endpoint, bucket: bucket, accessKey: accessKey,
		secretKey: secretKey, region: region,
		client: &http.Client{Timeout: 15 * time.Minute},
	}
}

func archiveObjectKey(studyUID, kind, name string) string {
	return "viewer-xa/" + studyUID + "/" + kind + "/" + name
}

func archiveManifestKey(studyUID string) string {
	return "viewer-xa/" + studyUID + "/manifest.json"
}

func escapeObjectKey(key string) string {
	parts := strings.Split(key, "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func (storage *yandexArchive) request(
	ctx context.Context,
	method, key, contentType, payloadHash string,
	body io.Reader,
) (*http.Response, error) {
	return storage.signedRequest(ctx, method, key, contentType, payloadHash, body, "")
}

func (storage *yandexArchive) signAndDo(request *http.Request, contentType, payloadHash string) (*http.Response, error) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	date := now.Format("20060102")
	host := request.URL.Host
	request.Header.Set("Host", host)
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := request.Method + "\n" + request.URL.EscapedPath() + "\n\n" +
		canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	scope := date + "/" + storage.region + "/s3/aws4_request"
	canonicalDigest := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(canonicalDigest[:])
	dateKey := hmacSHA256([]byte("AWS4"+storage.secretKey), date)
	regionKey := hmacSHA256(dateKey, storage.region)
	serviceKey := hmacSHA256(regionKey, "s3")
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+storage.accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return storage.client.Do(request)
}

func fileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	return hex.EncodeToString(digest.Sum(nil)), size, err
}

func (storage *yandexArchive) putFile(ctx context.Context, key, path, contentType string) error {
	digest, size, err := fileSHA256(path)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	response, err := storage.request(ctx, http.MethodPut, key, contentType, digest, file)
	_ = file.Close()
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return fmt.Errorf("Yandex PUT %s: HTTP %d: %s", key, response.StatusCode, strings.TrimSpace(string(message)))
	}
	return storage.verify(ctx, key, size)
}

func (storage *yandexArchive) putBytes(ctx context.Context, key, contentType string, payload []byte) error {
	digest := sha256.Sum256(payload)
	response, err := storage.request(ctx, http.MethodPut, key, contentType, hex.EncodeToString(digest[:]), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Yandex PUT %s: HTTP %d", key, response.StatusCode)
	}
	return storage.verify(ctx, key, int64(len(payload)))
}

func (storage *yandexArchive) verify(ctx context.Context, key string, expected int64) error {
	empty := sha256.Sum256(nil)
	response, err := storage.request(ctx, http.MethodHead, key, "", hex.EncodeToString(empty[:]), nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Yandex HEAD %s: HTTP %d", key, response.StatusCode)
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil || size != expected {
		return fmt.Errorf("Yandex object %s size mismatch: got %d want %d", key, size, expected)
	}
	return nil
}

func (storage *yandexArchive) get(ctx context.Context, key string) (*http.Response, error) {
	return storage.getRange(ctx, key, "")
}

func (storage *yandexArchive) getRange(ctx context.Context, key, byteRange string) (*http.Response, error) {
	empty := sha256.Sum256(nil)
	response, err := storage.signedRequest(ctx, http.MethodGet, key, "", hex.EncodeToString(empty[:]), nil, byteRange)
	return response, err
}

func (storage *yandexArchive) signedRequest(ctx context.Context, method, key, contentType, payloadHash string, body io.Reader, byteRange string) (*http.Response, error) {
	objectURL := storage.endpoint + "/" + url.PathEscape(storage.bucket) + "/" + escapeObjectKey(key)
	request, err := http.NewRequestWithContext(ctx, method, objectURL, body)
	if err != nil {
		return nil, err
	}
	if byteRange != "" {
		request.Header.Set("Range", byteRange)
	}
	return storage.signAndDo(request, contentType, payloadHash)
}

func (storage *yandexArchive) readJSON(ctx context.Context, key string) ([]byte, error) {
	response, err := storage.get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Yandex GET %s: HTTP %d", key, response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 16<<20))
}

func (storage *yandexArchive) uploadStudy(ctx context.Context, root string, manifest xaCacheManifest) error {
	type upload struct {
		key         string
		path        string
		contentType string
	}
	uploads := make([]upload, 0, manifest.FrameCount+len(manifest.Series))
	for _, series := range manifest.Series {
		uploads = append(uploads, upload{
			key:         archiveObjectKey(manifest.StudyUID, "cines", series.CineID),
			path:        filepath.Join(root, manifest.StudyUID, "cines", series.CineID),
			contentType: "video/mp4",
		})
		for _, frame := range series.Frames {
			uploads = append(uploads, upload{
				key:         archiveObjectKey(manifest.StudyUID, "frames", frame.ID),
				path:        filepath.Join(root, manifest.StudyUID, "frames", frame.ID),
				contentType: "image/jpeg",
			})
		}
	}
	uploadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan upload)
	errCh := make(chan error, 1)
	var workers sync.WaitGroup
	workerCount := 6
	if len(uploads) < workerCount {
		workerCount = len(uploads)
	}
	for index := 0; index < workerCount; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				if err := storage.putFile(uploadCtx, item.key, item.path, item.contentType); err != nil {
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}
			}
		}()
	}
	for _, item := range uploads {
		select {
		case jobs <- item:
		case <-uploadCtx.Done():
		}
		if uploadCtx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	select {
	case err := <-errCh:
		return err
	default:
	}
	manifest.CloudArchived = true
	payload, err := jsonMarshal(manifest)
	if err != nil {
		return err
	}
	return storage.putBytes(ctx, archiveManifestKey(manifest.StudyUID), "application/json", payload)
}

func jsonMarshal(value any) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
