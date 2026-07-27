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
	for _, file := range files {
		if err := importRemotePACSFile(
			ctx,
			client,
			remoteURL,
			username,
			password,
			file,
		); err != nil {
			return err
		}
	}
	return nil
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

	temporaryFile, err := os.CreateTemp("", "viewer-dicom-*.dcm")
	if err != nil {
		_ = downloadResponse.Body.Close()
		return fmt.Errorf("create temporary DICOM file: %w", err)
	}
	temporaryPath := temporaryFile.Name()
	defer func() {
		_ = temporaryFile.Close()
		_ = os.Remove(temporaryPath)
	}()

	written, copyErr := io.Copy(temporaryFile, downloadResponse.Body)
	closeErr := downloadResponse.Body.Close()
	if copyErr != nil {
		return fmt.Errorf("download DICOM %s body: %w", file.Name, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close DICOM %s download: %w", file.Name, closeErr)
	}
	if written != file.Size {
		return fmt.Errorf(
			"download DICOM %s size mismatch: received=%d expected=%d",
			file.Name,
			written,
			file.Size,
		)
	}
	if _, err := temporaryFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind DICOM %s: %w", file.Name, err)
	}

	uploadRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		remoteURL,
		temporaryFile,
	)
	if err != nil {
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
	return nil
}
