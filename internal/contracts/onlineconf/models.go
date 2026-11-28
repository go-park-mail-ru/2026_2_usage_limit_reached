// Package onlineconf описывает контракты OnlineConf для сервисов-потребителей:
// интерфейс чтения, модель флага, результат вычисления и ошибки.
// Здесь нет gRPC и сети, поэтому usecase зависит только от Reader и легко
// тестируется подменой.
package onlineconf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	// ErrUnavailable — сервис OnlineConf недоступен или не ответил вовремя.
	// Клиент продолжает работать на последнем успешно загруженном снапшоте.
	ErrUnavailable = errors.New("onlineconf unavailable")

	// ErrInvalidScopes — сервис отклонил запрос из-за пустого списка скоупов.
	ErrInvalidScopes = errors.New("onlineconf: invalid scopes")

	// ErrEmptyResponse — сервис вернул пустой ответ без ошибки.
	ErrEmptyResponse = errors.New("onlineconf returned empty response")

	// ErrInvalidValue — значение настройки не разобралось в запрошенный тип.
	ErrInvalidValue = errors.New("onlineconf: invalid config value")
)

// Reader — то, от чего зависят usecase и delivery.
type Reader interface {
	// Value возвращает значение настройки в виде JSON-текста, как оно хранится
	// в oc_config.value (JSONB): `100`, `"abc"` (строка с кавычками), `true`,
	// `[1,2]` или `{"min":10,"max":1000}`. false — настройки нет в снапшоте
	// (не создана, скоуп не запрошен или снапшот ещё не загружен).
	// Разбирать значение удобнее хелперами JSON, String, Int64 и Bool.
	Value(scope, key string) (string, bool)

	// Enabled вычисляет флаг для пользователя. Анонимный пользователь
	// (uuid.Nil) и отсутствующий флаг дают false.
	Enabled(scope, key string, userID uuid.UUID) bool
}

// JSON разбирает значение настройки в T.
//
// Отсутствующая настройка даёт def и nil. Значение, которое не разобралось
// в T (битый JSON, лишний хвост, другой тип, число вне диапазона, null), даёт def
// и ошибку, обёрнутую в ErrInvalidValue: отравленный конфиг не должен ломать
// запросы, а вызывающий код при желании логирует ошибку.
//
// Значение разбирается в новый нулевой T, а не в копию def: def остаётся
// нетронутым даже для map и slice. Поля структуры, которых нет в JSON, получают
// нулевые значения, поэтому результат нужно валидировать (например, min > 0).
func JSON[T any](r Reader, scope, key string, def T) (T, error) {
	raw, ok := r.Value(scope, key)
	if !ok {
		return def, nil
	}

	if bytes.Equal(bytes.TrimSpace([]byte(raw)), []byte("null")) {
		return def, fmt.Errorf("%w: %s/%s: null is not allowed", ErrInvalidValue, scope, key)
	}

	var decoded T
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return def, fmt.Errorf("%w: %s/%s: %v", ErrInvalidValue, scope, key, err)
	}

	return decoded, nil
}

// String возвращает JSON-строку или def. Число, булево и прочие типы дают def.
// Для логирования ошибки используйте JSON[string].
func String(r Reader, scope, key, def string) string {
	value, _ := JSON(r, scope, key, def)
	return value
}

// Int64 возвращает JSON-целое или def. Дробное число (1.5), число в виде строки ("100"),
// переполнение и прочие типы дают def. Для логирования ошибки используйте JSON[int64].
func Int64(r Reader, scope, key string, def int64) int64 {
	value, _ := JSON(r, scope, key, def)
	return value
}

// Bool возвращает JSON-булево или def. Строка "true" не считается булевым значением.
// Для логирования ошибки используйте JSON[bool].
func Bool(r Reader, scope, key string, def bool) bool {
	value, _ := JSON(r, scope, key, def)
	return value
}

// Flag — тоггл в том виде, в котором его вычисляет потребитель.
type Flag struct {
	Scope string
	Key   string

	// RolloutPercent — доля пользователей с включённым флагом, 0..100.
	RolloutPercent uint32

	// ForceOn и ForceOff — принудительные переопределения для пользователей.
	ForceOn  map[uuid.UUID]struct{}
	ForceOff map[uuid.UUID]struct{}
}

// Reason объясняет результат вычисления флага. Используется в логах и в
// админской проверке «почему у юзера включено/выключено».
type Reason string

const (
	ReasonFlagNotFound Reason = "flag_not_found"
	ReasonAnonymous    Reason = "anonymous"
	ReasonForceOff     Reason = "force_off"
	ReasonForceOn      Reason = "force_on"
	ReasonRollout      Reason = "rollout"
	ReasonNotInRollout Reason = "not_in_rollout"
)

// Decision — результат вычисления флага для пользователя.
type Decision struct {
	Enabled bool
	Reason  Reason
}
