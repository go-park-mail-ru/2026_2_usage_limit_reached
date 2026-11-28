package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
)

// FeedSettings отдаёт состояние расширенных фильтров ленты для текущего
// пользователя. Гость (guest) получает выключенные фильтры: анонимным
// пользователям раскатка не применяется (uuid.Nil всегда вне раскатки).
func (uc *ContentUsecase) FeedSettings(ctx context.Context, userID *uuid.UUID) *dto.FeedSettings {
	_ = ctx

	return &dto.FeedSettings{
		AdvancedFiltersEnabled: uc.feedAttachmentFilterEnabled(userID),
		// «Доступно мне» бэкенд пока не поддерживает.
		AccessibleOnlySupported: false,
	}
}

func (uc *ContentUsecase) feedAttachmentFilterEnabled(userID *uuid.UUID) bool {
	if uc.onlineconf == nil {
		return false
	}

	id := uuid.Nil
	if userID != nil {
		id = *userID
	}

	return uc.onlineconf.Enabled(feedAttachmentFilterScope, feedAttachmentFilterKey, id)
}

// checkFeedAttachmentFilter пускает запрос ленты с фильтром по вложениям только
// если флаг включён для пользователя. Выключенный флаг на валидный
// attachment_type отвечает ошибкой (её delivery превращает в 403), чтобы фронт
// сбросил фильтр и перезагрузил первую страницу, а не получал молча
// отфильтрованную ленту.
func (uc *ContentUsecase) checkFeedAttachmentFilter(userID *uuid.UUID, filters dto.FeedFilters) error {
	if filters.AttachmentType == "" {
		return nil
	}

	if !uc.feedAttachmentFilterEnabled(userID) {
		return fmt.Errorf("feed attachment filter: %w", ErrAdvancedFiltersDisabled)
	}

	return nil
}
