package mock_delivery_http

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/google/uuid"
)

// FeedSettings provides a mock function for the type MockContentUsecase.
//
// Метод интерфейса добавился после генерации мока, поэтому до следующего
// прогона mockery он живёт отдельным файлом (тот же пакет, без дублей).
func (_mock *MockContentUsecase) FeedSettings(ctx context.Context, userID *uuid.UUID) *dto.FeedSettings {
	ret := _mock.Called(ctx, userID)

	if len(ret) == 0 {
		panic("no return value specified for FeedSettings")
	}

	var r0 *dto.FeedSettings
	if returnFunc, ok := ret.Get(0).(func(context.Context, *uuid.UUID) *dto.FeedSettings); ok {
		r0 = returnFunc(ctx, userID)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).(*dto.FeedSettings)
	}
	return r0
}
