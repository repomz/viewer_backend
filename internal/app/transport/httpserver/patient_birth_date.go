package httpserver

import (
	"github.com/repomz/viewer_backend/internal/app/domain"
	"time"
)

func parseStudyBirthDate(value string) time.Time {
	date, _ := time.Parse("2006-01-02", value)
	return date
}

func studyBirthDateString(study domain.Study) string {
	if value := study.BirthDate(); value.Valid {
		return value.Time.Format("2006-01-02")
	}
	return ""
}

// Compare calendar dates, not durations: birthdays and leap years matter.
func ageOnDate(birth, date time.Time) int {
	age := date.Year() - birth.Year()
	if date.Month() < birth.Month() || (date.Month() == birth.Month() && date.Day() < birth.Day()) {
		age--
	}
	return age
}

func planBirthMatches(value string, study domain.Study) bool {
	birth := parseStudyBirthDate(value)
	if birth.IsZero() {
		return false
	}
	if actual := study.BirthDate(); actual.Valid {
		return actual.Time.Format("2006-01-02") == value
	}
	operation := study.TimeBeginning()
	age := study.Age()
	return operation.Valid && age.Valid && ageOnDate(birth, operation.Time.In(time.Local)) == int(age.Int32)
}
