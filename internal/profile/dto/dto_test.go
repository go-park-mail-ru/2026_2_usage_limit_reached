package dto_test

import (
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestUpdateProfileRequest_Validate(t *testing.T) {
	v := testutils.SetupValidator()

	tests := []struct {
		name    string
		req     dto.UpdateProfileRequest
		wantErr bool
	}{
		{name: "Valid without author", req: dto.UpdateProfileRequest{Nickname: "Nick"}},
		{
			name: "Valid with author",
			req:  dto.UpdateProfileRequest{Nickname: "Nick", Author: &dto.UpdateAuthorRequest{Bio: "bio", Category: ptr("tech")}},
		},
		{
			name: "Valid author without category",
			req:  dto.UpdateProfileRequest{Nickname: "Nick", Author: &dto.UpdateAuthorRequest{Bio: "bio"}},
		},
		{name: "Empty nickname", req: dto.UpdateProfileRequest{Nickname: "  "}, wantErr: true},
		{
			name:    "Empty bio",
			req:     dto.UpdateProfileRequest{Nickname: "Nick", Author: &dto.UpdateAuthorRequest{Bio: ""}},
			wantErr: true,
		},
		{
			name: "Too long category",
			req: dto.UpdateProfileRequest{
				Nickname: "Nick",
				Author:   &dto.UpdateAuthorRequest{Bio: "bio", Category: ptr(strings.Repeat("a", 1000))},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate(v)

			if tt.wantErr {
				require.ErrorIs(t, err, validator.ErrValidation)
				return
			}
			require.NoError(t, err)
		})
	}
}
