// Package models — доменные модели OnlineConf.
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrScopeNotFound = errors.New("scope not found")
	ErrScopeExists   = errors.New("scope already exists")
	ErrConfigNotFound = errors.New("config not found")
	ErrConfigExists  = errors.New("config already exists")
	ErrFlagNotFound  = errors.New("flag not found")
	ErrFlagExists    = errors.New("flag already exists")
	ErrInvalidValue  = errors.New("invalid config value")
)

// Форматы значения конфига из задания: json/массив/простое значение.
const (
	ValueTypeScalar = "scalar"
	ValueTypeArray  = "array"
	ValueTypeJSON   = "json"
)

// Роль и статус пользователя, влияющие на доступ к админке.
// Проверка читает их живым SELECT'ом из "user" (дизайн, решение 16):
// токен отвечает за идентификацию, БД — за актуальные права.
const (
	RoleAdmin     = "admin"
	StatusActive  = "active"
)

type Scope struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Config — значение настройки. Value — JSONB в виде текста (JSON-строка):
// потребители разбирают его сами, value_type — подсказка валидации и редактора.
type Config struct {
	ID          uuid.UUID
	Scope       string
	Key         string
	Value       string
	ValueType   string
	Description string
	UpdatedBy   *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Flag — тоггл с процентной раскаткой и принудительными переопределениями.
type Flag struct {
	ID             uuid.UUID
	Scope          string
	Key            string
	Description    string
	RolloutPercent int16
	UpdatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ForceOn        []uuid.UUID
	ForceOff       []uuid.UUID
}

type HistoryEntry struct {
	ID         int64
	ActorID    *uuid.UUID
	EntityType string
	EntityID   uuid.UUID
	Action     string
	OldValue   *string
	NewValue   *string
	CreatedAt  time.Time
}

// HistoryFilter — фильтр списка истории. EntityType нужен только вместе
// со Scope/Key: без них история отдается целиком.
type HistoryFilter struct {
	EntityType string
	Scope      string
	Key        string
}

// Snapshot — срез скоупов с ревизией. NotModified=true означает, что
// known_revision актуален и данных в ответе нет.
type Snapshot struct {
	Revision    int64
	NotModified bool
	Configs     []Config
	Flags       []Flag
}
