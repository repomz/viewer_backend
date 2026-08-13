package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"github.com/repomz/viewer_backend/internal/app/transport/httpmodels"
)

func (h HttpServer) CreateCTStudy(w http.ResponseWriter, r *http.Request) {
	h.createModalityStudy(w, r, "CT")
}

func (h HttpServer) CreateXAStudy(w http.ResponseWriter, r *http.Request) {
	h.createModalityStudy(w, r, "XA")
}

func (h HttpServer) createModalityStudy(
	w http.ResponseWriter,
	r *http.Request,
	modality string,
) {
	var request httpmodels.ModalityStudyRequest
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	if err := request.Validate(modality); err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	existing, err := h.studyService.GetStudyByStudyIDAndType(
		r.Context(),
		request.StudyUID,
		strings.ToLower(modality),
	)
	if err == nil {
		if r.URL.Query().Get("force_pacs") == "true" {
			if err := importRemotePACS(r.Context(), request.DicomFiles); err != nil {
				server.BadGateway("remote-pacs-import", err, w, r)
				return
			}
		}
		if modality == "XA" && h.xaCache != nil {
			h.xaCache.Enqueue(request.StudyUID)
		}
		server.RespondOK(toResponseStudy(existing), w, r)
		return
	}
	if !errors.Is(err, domain.ErrNotFound) {
		server.RespondWithError(err, w, r)
		return
	}
	if err := importRemotePACS(r.Context(), request.DicomFiles); err != nil {
		server.BadGateway("remote-pacs-import", err, w, r)
		return
	}

	timeBeginning := parseDICOMDateTime(request.StudyDate, request.StudyTime)
	description := request.Description
	if description == "" {
		description = modality
	}
	study := domain.ResponseToDBStudy(domain.DBStudyData{
		StudyID:        request.StudyUID,
		Patient:        request.Patient,
		Age:            request.Age,
		Department:     "не указано",
		NameOperation:  description,
		StudyType:      strings.ToLower(modality),
		DescrOperation: description,
		TimeBeginning:  timeBeginning,
		Surgeon:        "не указано",
		DicomLink:      request.DicomLink,
	})
	inserted, err := h.studyService.CreateStudy(r.Context(), study)
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}
	log.Printf(
		"remote PACS import completed: modality=%s study_uid=%s files=%d",
		modality,
		request.StudyUID,
		len(request.DicomFiles),
	)
	if modality == "XA" && h.xaCache != nil {
		h.xaCache.Enqueue(request.StudyUID)
	}
	server.RespondCreated(toResponseStudy(inserted), w, r)
}

func parseDICOMDateTime(dateValue, timeValue string) time.Time {
	timeValue = strings.SplitN(timeValue, ".", 2)[0]
	timeValue += strings.Repeat("0", max(0, 6-len(timeValue)))
	parsed, err := time.ParseInLocation(
		"20060102150405",
		dateValue+timeValue[:min(6, len(timeValue))],
		time.Local,
	)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func importRemotePACS(ctx context.Context, files []httpmodels.DicomFile) error {
	remoteURL := strings.TrimSpace(os.Getenv("REMOTE_PACS_URL"))
	if remoteURL == "" {
		return fmt.Errorf("REMOTE_PACS_URL is not configured")
	}
	timeoutSeconds, err := strconv.Atoi(os.Getenv("REMOTE_PACS_TIMEOUT_SECONDS"))
	if err != nil || timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	username := os.Getenv("REMOTE_PACS_USERNAME")
	password := os.Getenv("REMOTE_PACS_PASSWORD")
	// Large XA instances can contain hundreds of uncompressed frames. Orthanc
	// temporarily needs considerably more memory than the DICOM file size while
	// storing such an instance, so imports are deliberately serial by default.
	workerCount := min(envPositiveInt("REMOTE_PACS_IMPORT_WORKERS", 1), len(files))
	if workerCount == 0 {
		return nil
	}
	workContext, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan httpmodels.DicomFile)
	errorsChannel := make(chan error, 1)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for file := range jobs {
				if err := importRemotePACSFile(
					workContext, client, remoteURL, username, password, file,
				); err != nil {
					select {
					case errorsChannel <- err:
						cancel()
					default:
					}
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, file := range files {
			select {
			case jobs <- file:
			case <-workContext.Done():
				return
			}
		}
	}()
	workers.Wait()
	select {
	case err := <-errorsChannel:
		return err
	default:
		return nil
	}
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	read, err := r.reader.Read(buffer)
	r.read += int64(read)
	return read, err
}

func importRemotePACSFile(
	ctx context.Context,
	client *http.Client,
	remoteURL string,
	username string,
	password string,
	file httpmodels.DicomFile,
) error {
	downloadRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return fmt.Errorf("build DICOM download request: %w", err)
	}
	downloadResponse, err := client.Do(downloadRequest)
	if err != nil {
		return fmt.Errorf("download DICOM %s: %w", file.Name, err)
	}
	if downloadResponse.StatusCode < 200 || downloadResponse.StatusCode >= 300 {
		_ = downloadResponse.Body.Close()
		return fmt.Errorf("download DICOM %s: HTTP %d", file.Name, downloadResponse.StatusCode)
	}

	if downloadResponse.ContentLength >= 0 && downloadResponse.ContentLength != file.Size {
		_ = downloadResponse.Body.Close()
		return fmt.Errorf(
			"download DICOM %s size mismatch: received=%d expected=%d",
			file.Name,
			downloadResponse.ContentLength,
			file.Size,
		)
	}
	counted := &countingReader{reader: downloadResponse.Body}
	body := struct {
		io.Reader
		io.Closer
	}{Reader: counted, Closer: downloadResponse.Body}

	uploadRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		remoteURL,
		body,
	)
	if err != nil {
		_ = downloadResponse.Body.Close()
		return fmt.Errorf("build remote PACS request: %w", err)
	}
	uploadRequest.ContentLength = file.Size
	uploadRequest.Header.Set("Content-Type", "application/dicom")
	if username != "" || password != "" {
		uploadRequest.SetBasicAuth(username, password)
	}
	uploadResponse, err := client.Do(uploadRequest)
	if err != nil {
		return fmt.Errorf("upload DICOM %s: %w", file.Name, err)
	}
	_, _ = io.Copy(io.Discard, uploadResponse.Body)
	_ = uploadResponse.Body.Close()
	if uploadResponse.StatusCode < 200 || uploadResponse.StatusCode >= 300 {
		return fmt.Errorf("upload DICOM %s: HTTP %d", file.Name, uploadResponse.StatusCode)
	}
	if counted.read != file.Size {
		return fmt.Errorf(
			"download DICOM %s size mismatch: received=%d expected=%d",
			file.Name, counted.read, file.Size,
		)
	}
	return nil
}
