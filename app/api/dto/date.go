package dto

import (
	"fmt"
	"strings"
	"time"
)

// Date -- дата без времени. В JSON сериализуется и разбирается
// в формате DateFormat ("2006-01-02"), в коде ведёт себя как time.Time.
type Date struct {
	time.Time
}

// UnmarshalJSON разбирает дату вида "2006-01-02".
// JSON null и пустая строка оставляют нулевое значение (валидацию делает домен).
func (d *Date) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		return nil
	}

	t, err := time.Parse(DateFormat, s)
	if err != nil {
		return fmt.Errorf("некорректный формат даты, ожидается %q: %w", DateFormat, err)
	}
	d.Time = t
	return nil
}

// MarshalJSON сериализует дату в формат "2006-01-02".
func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Time.Format(DateFormat) + `"`), nil
}
