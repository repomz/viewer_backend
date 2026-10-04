package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func TestOperationStatisticsCachesRepeatedReads(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	invalidateOperationStatisticsResponseCache()
	service := &studyServiceStub{}
	handler := NewHttpServer(service, nil)
	for range 2 {
		request := httptest.NewRequest(http.MethodGet, "/statistics/operations", nil)
		recorder := httptest.NewRecorder()
		handler.GetOperationStatistics(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
	if service.getAllCalls != 1 {
		t.Fatalf("study loads = %d, want 1", service.getAllCalls)
	}
}

func TestExpandedStatisticsTypesHaveOnePrimaryColumn(t *testing.T) {
	cases := map[string]string{
		"стент_па": "stent_pa", "ВЭКС": "pacer_temporary", "ЭКС 1к": "pacer_single",
		"ЭКС 2к": "pacer_dual", "ЭКС ревизия": "pacer_revision", "ВАБК": "iabp",
		"эма": "ema", "бап_периферии": "bap_peripheral", "ангиография": "angiography",
		"эмболизация": "embolization", "фистулография": "fistulography", "неизвестный тип": "other",
	}
	for kind, expected := range cases {
		study := domain.ResponseToDBStudy(domain.DBStudyData{StudyType: kind})
		ids := statisticsOperationTypeIDs(study)
		if len(ids) != 1 || ids[0] != expected {
			t.Fatalf("%s: got %v, expected %s", kind, ids, expected)
		}
	}
}

func statisticsStudy(id uuid.UUID, patient, operationType, surgeon string) domain.Study {
	return domain.ResponseToDBStudy(domain.DBStudyData{
		ID: id, StudyID: id.String(), Patient: patient, NameOperation: operationType,
		StudyType: operationType, Surgeon: surgeon,
		TimeBeginning: time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local),
	})
}

func TestBuildOperationStatisticsUsesConfirmedPaymentOnly(t *testing.T) {
	stentIncluded := uuid.New()
	stentExcluded := uuid.New()
	manualKAG := uuid.New()
	studies := []domain.Study{
		domain.ResponseToDBStudy(domain.DBStudyData{ID: stentIncluded, StudyType: "стент_кор", Options: "ivus", Payment: "vmp", Surgeon: "Идрисов", TimeBeginning: time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local)}),
		statisticsStudy(stentExcluded, "Петров", "стент_кор", "Идрисов"),
		domain.ResponseToDBStudy(domain.DBStudyData{ID: manualKAG, StudyType: "каг", Payment: "vmp", Surgeon: "Старков", TimeBeginning: time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local)}),
	}
	handler := NewHttpServer(&studyServiceStub{}, nil)
	result := handler.buildOperationStatistics(studies, vmpStatisticsConfig{
		OperationTypes:  []string{"stent_cor"},
		IncludedStudies: []string{manualKAG.String()},
		ExcludedStudies: []string{stentExcluded.String()},
	}, 2026)

	if len(result.OperationTypes) != len(statisticsOperationTypes) || len(result.VMPPatients) != 2 {
		t.Fatalf("unexpected statistics: %#v", result)
	}
	if result.Surgeons[0].Surgeon != "Идрисов" || result.Surgeons[0].Total != 2 || result.Surgeons[0].VMP != 1 {
		t.Fatalf("unexpected first surgeon row: %#v", result.Surgeons[0])
	}
	if result.Surgeons[1].Surgeon != "Старков" || result.Surgeons[1].VMP != 1 {
		t.Fatalf("unexpected second surgeon row: %#v", result.Surgeons[1])
	}
}

func TestStatisticsRecognizesStentWithIntravascularImaging(t *testing.T) {
	study := domain.ResponseToDBStudy(domain.DBStudyData{StudyType: "стент_кор", Options: "ivus"})
	ids := statisticsOperationTypeIDs(study)
	want := map[string]bool{"vzuzi": true, "stent_cor": true}
	for _, id := range ids {
		delete(want, id)
	}
	if len(want) != 0 {
		t.Fatalf("missing classifications %v in %v", want, ids)
	}
	for _, id := range ids {
		if id == "stent_vzuzi" {
			t.Fatalf("duplicate stent+VZUZI classification remains in %v", ids)
		}
	}
}

func TestBuildOperationStatisticsOnlyCountsRequestedYear(t *testing.T) {
	current := statisticsStudy(uuid.New(), "Иванов", "каг", "Идрисов")
	old := domain.ResponseToDBStudy(domain.DBStudyData{
		ID: uuid.New(), StudyID: uuid.NewString(), Patient: "Петров",
		NameOperation: "каг", StudyType: "каг", Surgeon: "Идрисов",
		TimeBeginning: time.Date(2025, 8, 3, 10, 0, 0, 0, time.Local),
	})
	result := (HttpServer{}).buildOperationStatistics(
		[]domain.Study{current, old}, vmpStatisticsConfig{}, 2026,
	)
	if len(result.Surgeons) != 1 || result.Surgeons[0].Total != 1 {
		t.Fatalf("unexpected yearly statistics: %#v", result)
	}
}

func TestVMPStatisticsConfigRoundTrip(t *testing.T) {
	t.Setenv("PLANS_DIR", t.TempDir())
	input := vmpStatisticsConfig{
		OperationTypes: []string{"каг", "каг"}, IncludedStudies: []string{"study-1"},
		ExcludedStudies: []string{},
	}
	if err := saveVMPStatisticsConfig(input); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadVMPStatisticsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.OperationTypes) != 1 || loaded.OperationTypes[0] != "каг" || loaded.IncludedStudies[0] != "study-1" {
		t.Fatalf("unexpected config: %#v", loaded)
	}
}
