package memory

import (
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
)

func NewProfileRepositoryWithMockData(authors map[uuid.UUID]models.Author, posts []models.Post) (*ProfileRepository, error) {
	repo := NewProfileRepository()

	for userID, author := range authors {
		if userID == uuid.Nil {
			return nil, fmt.Errorf("mock author has an empty user ID")
		}
		repo.authors[userID] = author
	}

	for _, post := range posts {
		if post.ID == uuid.Nil {
			return nil, fmt.Errorf("mock post has an empty ID")
		}
		if _, exists := repo.authors[post.AuthorID]; !exists {
			return nil, fmt.Errorf("mock post %s has no author", post.ID)
		}
		if _, exists := repo.posts[post.ID]; exists {
			return nil, fmt.Errorf("duplicate mock post ID %s", post.ID)
		}
		repo.posts[post.ID] = post
	}

	return repo, nil
}
