package validator_test

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig_MediaRules(t *testing.T) {
	t.Parallel()

	media := validator.DefaultConfig().Media

	require.Equal(t, int64(25<<20), media.MaxImageSize)
	require.Equal(t, int64(1<<30), media.MaxVideoSize)
	require.Equal(t, int64(100<<20), media.MaxAudioSize)
	require.Equal(t, int64(100<<20), media.MaxFileSize)
	require.Equal(t, 20, media.MaxPostAttachments)
	require.Equal(t, 20, media.MaxPendingMedia)
}

func TestValidationConfig_Validate_Media(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(cfg *validator.ValidationConfig)
		errMsg string
	}{
		{
			name:   "image size is not positive",
			mutate: func(cfg *validator.ValidationConfig) { cfg.Media.MaxImageSize = 0 },
			errMsg: "media.max_image_size",
		},
		{
			name:   "video size is not positive",
			mutate: func(cfg *validator.ValidationConfig) { cfg.Media.MaxVideoSize = -1 },
			errMsg: "media.max_video_size",
		},
		{
			name:   "audio size is not positive",
			mutate: func(cfg *validator.ValidationConfig) { cfg.Media.MaxAudioSize = 0 },
			errMsg: "media.max_audio_size",
		},
		{
			name:   "file size is not positive",
			mutate: func(cfg *validator.ValidationConfig) { cfg.Media.MaxFileSize = 0 },
			errMsg: "media.max_file_size",
		},
		{
			name:   "post attachments limit is not positive",
			mutate: func(cfg *validator.ValidationConfig) { cfg.Media.MaxPostAttachments = 0 },
			errMsg: "media.max_post_attachments",
		},
		{
			name:   "pending media limit is not positive",
			mutate: func(cfg *validator.ValidationConfig) { cfg.Media.MaxPendingMedia = 0 },
			errMsg: "media.max_pending_media",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validator.DefaultConfig()
			tt.mutate(&cfg)

			require.ErrorContains(t, cfg.Validate(), tt.errMsg)
		})
	}

	t.Run("default config is valid", func(t *testing.T) {
		t.Parallel()

		_, err := validator.NewValidator(validator.DefaultConfig())

		require.NoError(t, err)
	})
}

func TestValidator_MediaLimits(t *testing.T) {
	t.Parallel()

	v := mustValidator(t)

	require.Equal(t, int64(25<<20), v.MaxImageSize())
	require.Equal(t, int64(1<<30), v.MaxVideoSize())
	require.Equal(t, int64(100<<20), v.MaxAudioSize())
	require.Equal(t, int64(100<<20), v.MaxFileSize())
	require.Equal(t, 20, v.MaxPostAttachments())
	require.Equal(t, 20, v.MaxPendingMedia())
}

func mustValidator(t *testing.T) *validator.Validator {
	t.Helper()

	v, err := validator.NewValidator(validator.DefaultConfig())
	require.NoError(t, err)

	return v
}
