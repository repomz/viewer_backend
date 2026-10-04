package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/repomz/viewer_backend/internal/app/common/server"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

type vmpStatisticsConfig struct {
	OperationTypes  []string `json:"operation_types"`
	IncludedStudies []string `json:"included_study_ids"`
	ExcludedStudies []string `json:"excluded_study_ids"`
}

type statisticsOperationType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Total int    `json:"total"`
}

type surgeonStatistics struct {
	Surgeon string         `json:"surgeon"`
	Counts  map[string]int `json:"counts"`
	VMP     int            `json:"vmp"`
	Total   int            `json:"total"`
}

type vmpPatient struct {
	StudyID       string `json:"study_id"`
	Patient       string `json:"patient"`
	Operation     string `json:"operation"`
	OperationType string `json:"operation_type"`
	Surgeon       string `json:"surgeon"`
	Date          string `json:"date"`
	Source        string `json:"source"`
}

type operationStatisticsResponse struct {
	OperationTypes    []statisticsOperationType `json:"operation_types"`
	Surgeons          []surgeonStatistics       `json:"surgeons"`
	VMPOperationTypes []string                  `json:"vmp_operation_types"`
	VMPPatients       []vmpPatient              `json:"vmp_patients"`
	IncludedStudyIDs  []string                  `json:"included_study_ids"`
	ExcludedStudyIDs  []string                  `json:"excluded_study_ids"`
}

var vmpStatisticsMu sync.RWMutex

func vmpStatisticsPath() string {
	directory := strings.TrimSpace(os.Getenv("PLANS_DIR"))
	if directory == "" {
		directory = strings.TrimSpace(os.Getenv("REPORTS_DIR"))
	}
	if directory == "" {
		directory = "/app/reports"
	}
	return filepath.Join(directory, "vmp-statistics.json")
}

func loadVMPStatisticsConfig() (vmpStatisticsConfig, error) {
	config := vmpStatisticsConfig{
		OperationTypes: []string{}, IncludedStudies: []string{}, ExcludedStudies: []string{},
	}
	file, err := os.Open(vmpStatisticsPath())
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return vmpStatisticsConfig{}, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return vmpStatisticsConfig{}, err
	}
	config.OperationTypes = normalizeOperationTypes(config.OperationTypes)
	config.IncludedStudies = normalizeStringSet(config.IncludedStudies)
	config.ExcludedStudies = normalizeStringSet(config.ExcludedStudies)
	return config, nil
}

func saveVMPStatisticsConfig(config vmpStatisticsConfig) error {
	path := vmpStatisticsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".vmp-statistics-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o640); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func normalizeStringSet(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeOperationTypes(values []string) []string {
	for index, value := range values {
		values[index] = strings.Join(strings.Fields(strings.ToLower(value)), "_")
	}
	return normalizeStringSet(values)
}

var statisticsOperationTypes = []statisticsOperationType{
	{ID: "vmp", Label: "ВМП"},
	{ID: "vzuzi", Label: "ВСУЗИ"},
	{ID: "vabk_option", Label: "ВАБК (доп.)"},
	{ID: "ekmo_option", Label: "ЭКМО (доп.)"},
	{ID: "ecmo", Label: "ЭКМО"},
	{ID: "angiography_peripheral", Label: "АНГИО ПЕРИФЕРИИ"},
	{ID: "embolization_peripheral", Label: "ЭМБОЛИЗАЦИЯ ПЕРИФЕРИИ"},
	{ID: "kag", Label: "КАГ"},
	{ID: "tsag", Label: "ЦАГ"},
	{ID: "stent_cor", Label: "СТЕНТ КОР"},
	{ID: "bap_cor", Label: "БАП КОР"},
	{ID: "stent_vsa", Label: "СТЕНТ ВСА"},
	{ID: "stent_pa", Label: "СТЕНТ ПА"},
	{ID: "stent_upper", Label: "СТЕНТ В/К"},
	{ID: "stent_lower", Label: "СТЕНТ Н/К"},
	{ID: "stent_renal", Label: "СТЕНТ ПОЧКИ"},
	{ID: "aneurysm", Label: "АНЕВРИЗМА"},
	{ID: "stroke", Label: "ИНСУЛЬТ"},
	{ID: "bap_shin", Label: "Голень"},
	{ID: "bap_peripheral", Label: "БАП ПЕРИФ"},
	{ID: "bap_carotid", Label: "БАП ВСА"},
	{ID: "bap_fistula", Label: "БАП ФИСТУЛЫ"},
	{ID: "ema", Label: "ЭМА"},
	{ID: "pacer_dual", Label: "ЭКС 2к"},
	{ID: "pacer_single", Label: "ЭКС 1к"},
	{ID: "pacer_temporary", Label: "ВЭКС"},
	{ID: "pacer_revision", Label: "ЭКС РЕВ"},
	{ID: "pacer_other", Label: "ЭКС ПРОЧ"},
	{ID: "iabp", Label: "ВАБК"},
	{ID: "aspiration", Label: "ТА/ТЭ"},
	{ID: "angiography", Label: "АНГИО"},
	{ID: "embolization", Label: "ЭМБОЛ"},
	{ID: "fistulography", Label: "ФИСТУЛОГР"},
	{ID: "stent_other", Label: "СТЕНТ ПРОЧ"},
	{ID: "bap_other", Label: "БАП ПРОЧ"},
	{ID: "other", Label: "ДРУГИЕ"},
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func statisticsOperationTypeIDs(study domain.Study) []string {
	text := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(study.StudyType())), " ", "_")
	mapping := map[string]string{"каг": "kag", "цаг": "tsag", "стент_кор": "stent_cor", "бап_кор": "bap_cor", "стент_вса": "stent_vsa", "стент_па": "stent_pa", "стент_вк": "stent_upper", "стент_нк": "stent_lower", "стент_почки": "stent_renal", "аневризма": "aneurysm", "инсульт": "stroke", "бап_голень": "bap_shin", "бап_периферии": "bap_peripheral", "бап_вса": "bap_carotid", "бап_фистулы": "bap_fistula", "эма": "ema", "экс_2к": "pacer_dual", "экс_1к": "pacer_single", "вэкс": "pacer_temporary", "экс_ревизия": "pacer_revision", "экс": "pacer_other", "экмо": "ecmo", "вабк": "iabp", "тромбаспирация": "aspiration", "ангиография": "angiography", "ангиография_периферии": "angiography_peripheral", "эмболизация": "embolization", "эмболизация_периферии": "embolization_peripheral", "фистулография": "fistulography", "стент_другие": "stent_other", "бап_другие": "bap_other", "другие": "other"}
	tags := []string{}
	if id := mapping[text]; id != "" {
		tags = append(tags, id)
	} else {
		tags = append(tags, "other")
	}
	for _, option := range strings.Split(strings.ReplaceAll(study.Options(), ";", ","), ",") {
		switch strings.ToLower(strings.TrimSpace(option)) {
		case "ivus":
			tags = append(tags, "vzuzi")
		case "vabk":
			if mapping[text] != "iabp" {
				tags = append(tags, "vabk_option")
			}
		case "ekmo":
			if mapping[text] != "ecmo" {
				tags = append(tags, "ekmo_option")
			}
		}
	}
	return normalizeStringSet(tags)
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func (h HttpServer) buildOperationStatistics(studies []domain.Study, config vmpStatisticsConfig, year int) operationStatisticsResponse {
	typeTotals := make(map[string]int)
	surgeons := make(map[string]*surgeonStatistics)
	vmpPatients := make([]vmpPatient, 0)

	for _, study := range studies {
		if !isProtocolStudy(study) {
			continue
		}
		beginning := study.TimeBeginning()
		if !beginning.Valid || beginning.Time.In(time.Local).Year() != year {
			continue
		}
		typeIDs := statisticsOperationTypeIDs(study)
		surgeonLabel := strings.TrimSpace(study.Surgeon())
		if surgeonLabel == "" {
			surgeonLabel = "Не указан"
		}
		surgeonKey := strings.ToLower(strings.ReplaceAll(surgeonLabel, "ё", "е"))
		row := surgeons[surgeonKey]
		if row == nil {
			row = &surgeonStatistics{Surgeon: surgeonLabel, Counts: make(map[string]int)}
			surgeons[surgeonKey] = row
		}
		for _, typeID := range typeIDs {
			typeTotals[typeID]++
			row.Counts[typeID]++
		}
		row.Total++

		studyID := study.ID().String()
		matchedVMPType := study.StudyType()
		isVMP := study.Payment() == "vmp" && !study.PaymentPending()
		if !isVMP {
			continue
		}
		row.VMP++
		row.Counts["vmp"]++
		typeTotals["vmp"]++
		date := ""
		if study.TimeBeginning().Valid {
			date = study.TimeBeginning().Time.In(time.Local).Format("2006-01-02")
		}
		source := "payment"
		vmpPatients = append(vmpPatients, vmpPatient{
			StudyID: studyID, Patient: study.Patient(), Operation: study.NameOperation(),
			OperationType: matchedVMPType, Surgeon: study.Surgeon(), Date: date, Source: source,
		})
	}

	types := make([]statisticsOperationType, 0, len(statisticsOperationTypes))
	for _, operationType := range statisticsOperationTypes {
		operationType.Total = typeTotals[operationType.ID]
		types = append(types, operationType)
	}
	rows := make([]surgeonStatistics, 0, len(surgeons))
	for _, row := range surgeons {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Total > rows[j].Total })
	sort.Slice(vmpPatients, func(i, j int) bool { return vmpPatients[i].Date > vmpPatients[j].Date })

	return operationStatisticsResponse{
		OperationTypes: types, Surgeons: rows, VMPOperationTypes: config.OperationTypes,
		VMPPatients: vmpPatients, IncludedStudyIDs: config.IncludedStudies,
		ExcludedStudyIDs: config.ExcludedStudies,
	}
}

func (h HttpServer) GetOperationStatistics(w http.ResponseWriter, r *http.Request) {
	year := time.Now().In(time.Local).Year()
	cacheKey := fmt.Sprintf("%s|%d", vmpStatisticsPath(), year)
	operationStatisticsResponseCache.Lock()
	if cached, ok := operationStatisticsResponseCache.entries[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
		operationStatisticsResponseCache.Unlock()
		server.RespondOK(cached.value, w, r)
		return
	}
	studies, err := loadStudiesForAnalysis(r.Context(), h.studyService)
	if err != nil {
		operationStatisticsResponseCache.Unlock()
		server.InternalError("statistics-studies", err, w, r)
		return
	}
	vmpStatisticsMu.RLock()
	config, err := loadVMPStatisticsConfig()
	vmpStatisticsMu.RUnlock()
	if err != nil {
		operationStatisticsResponseCache.Unlock()
		server.InternalError("statistics-config", err, w, r)
		return
	}
	response := h.buildOperationStatistics(studies, config, year)
	operationStatisticsResponseCache.entries[cacheKey] = cachedStatisticsResponse{
		value: response, expiresAt: time.Now().Add(statisticsCacheTTL),
	}
	operationStatisticsResponseCache.Unlock()
	server.RespondOK(response, w, r)
}

func (h HttpServer) PutVMPStatisticsConfig(w http.ResponseWriter, r *http.Request) {
	var request vmpStatisticsConfig
	if err := decodeJSON(w, r, &request); err != nil {
		server.BadRequest("invalid-json", err, w, r)
		return
	}
	request.OperationTypes = normalizeOperationTypes(request.OperationTypes)
	request.IncludedStudies = normalizeStringSet(request.IncludedStudies)
	request.ExcludedStudies = normalizeStringSet(request.ExcludedStudies)
	if len(request.OperationTypes) > 100 || len(request.IncludedStudies) > 5000 || len(request.ExcludedStudies) > 5000 {
		server.BadRequest("statistics-config-too-large", fmt.Errorf("too many VMP rules"), w, r)
		return
	}
	vmpStatisticsMu.Lock()
	err := saveVMPStatisticsConfig(request)
	vmpStatisticsMu.Unlock()
	if err != nil {
		server.InternalError("statistics-config", err, w, r)
		return
	}
	invalidateOperationStatisticsResponseCache()
	h.GetOperationStatistics(w, r)
}
