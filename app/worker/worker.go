package worker

import (
	"go.uber.org/zap"
)

// Type идентифицирует фоновый воркер в логах и в контексте вызова.
type Type string

const (
	TypeConfirmation Type = "confirmation"
	TypeCancellation Type = "cancellation"
)

func (t Type) String() string { return string(t) }

// LogField возвращает поле zap с типом воркера.
func (t Type) LogField() zap.Field { return zap.String("worker", t.String()) }
