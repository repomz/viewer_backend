package httpserver

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/repomz/viewer_backend/internal/app/domain"
)

func statisticsStudy(id uuid.UUID, patient, operationType, surgeon string) domain.Study {
	return domain.ResponseToDBStudy(domain.DBStudyData{
		ID: id, StudyID: id.String(), Patient: patient, NameOperation: operationType,
		StudyType: operationType, Surgeon: surgeon,
		TimeBeginning: time.Date(2026, 8, 3, 10, 0, 0, 0, time.Local),
	})
}

func TestBuildOperationStatisticsAppliesVMPTypesAndPatientOverrides(t *testing.T) {
	stentIncluded := uuid.New()
	stentExcluded := uuid.New()
	manualKAG := uuid.New()
	studies := []domain.Study{
		statisticsStudy(stentIncluded, "Иванов", "стент_кор", "Идрисов"),
		statisticsStudy(stentExcluded, "Петров", "стент_кор", "Идрисов"),
		statisticsStudy(manualKAG, "Сидоров", "каг", "Старков"),
	}
	handler := NewHttpServer(&studyServiceStub{}, nil)
	result := handler.buildOperationStatistics(studies, vmpStatisticsConfig{
		OperationTypes:  []string{"стент_кор"},
		IncludedStudies: []string{manualKAG.String()},
		ExcludedStudies: []string{stentExcluded.String()},
	})

	if len(result.OperationTypes) != 2 || len(result.VMPPatients) != 2 {
		t.Fatalf("unexpected statistics: %#v", result)
	}
	if result.Surgeons[0].Surgeon != "Идрисов" || result.Surgeons[0].Total != 2 || result.Surgeons[0].VMP != 1 {
		t.Fatalf("unexpected first surgeon row: %#v", result.Surgeons[0])
	}
	if result.Surgeons[1].Surgeon != "Старков" || result.Surgeons[1].VMP != 1 {
		t.Fatalf("unexpected second surgeon row: %#v", result.Surgeons[1])
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
