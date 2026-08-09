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
	{ID: "vzuzi", Label: "ВСУЗИ"},
	{ID: "kag", Label: "КАГ"},
	{ID: "tsag", Label: "ЦАГ"},
	{ID: "stent_cor", Label: "СТЕНТ КОР"},
	{ID: "stent_vzuzi", Label: "СТЕНТ + ВСУЗИ"},
	{ID: "bap_cor", Label: "БАП КОР"},
	{ID: "stent_vsa", Label: "СТЕНТ ВСА"},
	{ID: "stent_upper", Label: "СТЕНТ В/К"},
	{ID: "stent_lower", Label: "СТЕНТ Н/К"},
	{ID: "aneurysm", Label: "АНЕВРИЗМА"},
	{ID: "stroke", Label: "ИНСУЛЬТ"},
	{ID: "bap_shin", Label: "БАП ГОЛЕНЬ"},
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
	text := strings.ToLower(strings.ReplaceAll(strings.Join([]string{
		study.StudyType(), study.NameOperation(), study.DescrOperation(),
	}, " "), "ё", "е"))
	stent := containsAny(text, "стент", "чкв")
	vzuzi := containsAny(text, "всузи", "внутрисосудист")
	coronary := containsAny(text, "коронар", "каг", "чкв", "стент_кор", "бап_кор")
	tags := make([]string, 0, 3)
	if vzuzi {
		tags = append(tags, "vzuzi")
	}
	if stent && coronary {
		tags = append(tags, "stent_cor")
		if vzuzi {
			tags = append(tags, "stent_vzuzi")
		}
	} else if coronary && containsAny(text, "баллон", "бап") {
		tags = append(tags, "bap_cor")
	} else if containsAny(text, "каг", "коронарограф") {
		tags = append(tags, "kag")
	}
	if !stent && containsAny(text, "цаг", "церебраль", "ангиограф") && containsAny(text, "бца", "церебраль", "цаг") {
		tags = append(tags, "tsag")
	}
	if stent && containsAny(text, "вса", "каротид", "сонн") {
		tags = append(tags, "stent_vsa")
	}
	if stent && containsAny(text, "верхн", "подключ") {
		tags = append(tags, "stent_upper")
	}
	if stent && containsAny(text, "нижн", "опа", "нпа", "бедрен", "подкол") {
		tags = append(tags, "stent_lower")
	}
	if containsAny(text, "эмболизац") && containsAny(text, "аневризм") {
		tags = append(tags, "aneurysm")
	}
	if containsAny(text, "тромбаспирац", "тромбэкстракц", "тромбэктом") && containsAny(text, "сма", "пма", "зма", "базиляр", "вса", "инсульт") {
		tags = append(tags, "stroke")
	}
	if containsAny(text, "бап голен", "баллонная ангиопластика голен") {
		tags = append(tags, "bap_shin")
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
	vmpTypes := stringSet(config.OperationTypes)
	included := stringSet(config.IncludedStudies)
	excluded := stringSet(config.ExcludedStudies)
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
		matchedVMPType := ""
		for _, typeID := range typeIDs {
			if vmpTypes[typeID] {
				matchedVMPType = typeID
				break
			}
		}
		isVMP := !excluded[studyID] && (matchedVMPType != "" || included[studyID])
		if !isVMP {
			continue
		}
		row.VMP++
		date := ""
		if study.TimeBeginning().Valid {
			date = study.TimeBeginning().Time.In(time.Local).Format("2006-01-02")
		}
		source := "type"
		if included[studyID] {
			source = "patient"
		}
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
	studies, err := loadStudiesForAnalysis(r.Context(), h.studyService)
	if err != nil {
		server.InternalError("statistics-studies", err, w, r)
		return
	}
	vmpStatisticsMu.RLock()
	config, err := loadVMPStatisticsConfig()
	vmpStatisticsMu.RUnlock()
	if err != nil {
		server.InternalError("statistics-config", err, w, r)
		return
	}
	server.RespondOK(h.buildOperationStatistics(studies, config, time.Now().In(time.Local).Year()), w, r)
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
	h.GetOperationStatistics(w, r)
}
