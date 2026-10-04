package httpserver

import (
	"context"
	"github.com/repomz/viewer_backend/internal/app/domain"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestPaymentCannotBeChangedByRegularUser(t *testing.T) {
	for _, user := range []authenticatedUser{{Role: "user", Login: "marat"}, {Role: "user", Login: "artem"}} {
		r := httptest.NewRequest("PUT", "/studies/id/payment", nil)
		r = r.WithContext(context.WithValue(r.Context(), userContextKey{}, user))
		w := httptest.NewRecorder()
		(HttpServer{}).PutStudyPayment(w, r)
		if w.Code != 403 {
			t.Fatalf("status=%d", w.Code)
		}
	}
}

func TestStatisticsNeverInfersFromDescription(t *testing.T) {
	s := domain.ResponseToDBStudy(domain.DBStudyData{StudyType: "каг", NameOperation: "КАГ", DescrOperation: "Старый стент проходим. Рекомендовано ВСУЗИ."})
	if ids := statisticsOperationTypeIDs(s); !reflect.DeepEqual(ids, []string{"kag"}) {
		t.Fatal(ids)
	}
	s = domain.ResponseToDBStudy(domain.DBStudyData{StudyType: "стент_кор", Options: "ivus", Payment: "vmp", PaymentPending: true, TimeBeginning: time.Date(2026, 1, 2, 9, 0, 0, 0, time.Local)})
	result := (HttpServer{}).buildOperationStatistics([]domain.Study{s}, vmpStatisticsConfig{}, 2026)
	if result.Surgeons[0].VMP != 0 || result.Surgeons[0].Total != 1 {
		t.Fatal(result)
	}
}
