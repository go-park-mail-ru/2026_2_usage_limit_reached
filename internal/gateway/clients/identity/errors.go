package identityclient

import (
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/apperrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %v", apperrors.ErrInternal, err)
	}

	switch st.Code() {
	case codes.InvalidArgument:
		if st.Message() == "unsupported avatar type" {
			return fmt.Errorf("%w: %s", apperrors.ErrUnsupportedMediaType, st.Message())
		}
		return fmt.Errorf("%w: %s", apperrors.ErrBadRequest, st.Message())
	case codes.Unauthenticated:
		return fmt.Errorf("%w: %s", apperrors.ErrUnauthorized, st.Message())
	case codes.PermissionDenied:
		return fmt.Errorf("%w: %s", apperrors.ErrForbidden, st.Message())
	case codes.NotFound:
		return fmt.Errorf("%w: %s", apperrors.ErrNotFound, st.Message())
	case codes.AlreadyExists:
		return fmt.Errorf("%w: %s", apperrors.ErrConflict, st.Message())
	case codes.ResourceExhausted:
		return fmt.Errorf("%w: %s", apperrors.ErrPayloadTooLarge, st.Message())
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return fmt.Errorf("%w: %s", apperrors.ErrServiceUnavailable, st.Message())
	default:
		return fmt.Errorf("%w: %s: %s", apperrors.ErrInternal, st.Code(), st.Message())
	}
}
