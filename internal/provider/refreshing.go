// internal/provider/refreshing.go
package provider

import (
	"errors"
	"fmt"

	"github.com/waabox/gitdeck/internal/domain"
)

// AuthExpiredError is returned when both the access token and refresh token are
// invalid, and interactive re-authentication is required.
type AuthExpiredError struct {
	Provider string
}

func (e *AuthExpiredError) Error() string {
	return fmt.Sprintf("%s session expired: re-authentication required", e.Provider)
}

// RefreshingProvider wraps a PipelineProvider and transparently handles 401 errors
// by attempting a silent token refresh. If refresh fails, it returns AuthExpiredError.
type RefreshingProvider struct {
	inner       domain.PipelineProvider
	provider    string
	refreshFn   func() (string, error)
	updateToken func(string)
}

// Ensure RefreshingProvider implements PipelineProvider.
var _ domain.PipelineProvider = (*RefreshingProvider)(nil)

// NewRefreshingProvider creates a RefreshingProvider.
// refreshFn is called on 401 to attempt a silent token refresh; returns new access token.
// updateToken is called after successful refresh to inject the new token into the adapter.
func NewRefreshingProvider(
	inner domain.PipelineProvider,
	providerName string,
	refreshFn func() (string, error),
	updateToken func(string),
) *RefreshingProvider {
	return &RefreshingProvider{
		inner:       inner,
		provider:    providerName,
		refreshFn:   refreshFn,
		updateToken: updateToken,
	}
}

func (rp *RefreshingProvider) handleUnauthorized(retry func() error) error {
	newToken, refreshErr := rp.refreshFn()
	if refreshErr != nil {
		return &AuthExpiredError{Provider: rp.provider}
	}
	rp.updateToken(newToken)
	return retry()
}

func (rp *RefreshingProvider) ListPipelines(repo domain.Repository) ([]domain.Pipeline, error) {
	result, err := rp.inner.ListPipelines(repo)
	if err != nil && errors.Is(err, domain.ErrUnauthorized) {
		var retryResult []domain.Pipeline
		retryErr := rp.handleUnauthorized(func() error {
			var e error
			retryResult, e = rp.inner.ListPipelines(repo)
			return e
		})
		if retryErr != nil {
			return nil, retryErr
		}
		return retryResult, nil
	}
	return result, err
}

func (rp *RefreshingProvider) GetPipeline(repo domain.Repository, id domain.PipelineID) (domain.Pipeline, error) {
	result, err := rp.inner.GetPipeline(repo, id)
	if err != nil && errors.Is(err, domain.ErrUnauthorized) {
		var retryResult domain.Pipeline
		retryErr := rp.handleUnauthorized(func() error {
			var e error
			retryResult, e = rp.inner.GetPipeline(repo, id)
			return e
		})
		if retryErr != nil {
			return domain.Pipeline{}, retryErr
		}
		return retryResult, nil
	}
	return result, err
}

func (rp *RefreshingProvider) GetJobLogs(repo domain.Repository, jobID domain.JobID) (string, error) {
	result, err := rp.inner.GetJobLogs(repo, jobID)
	if err != nil && errors.Is(err, domain.ErrUnauthorized) {
		var retryResult string
		retryErr := rp.handleUnauthorized(func() error {
			var e error
			retryResult, e = rp.inner.GetJobLogs(repo, jobID)
			return e
		})
		if retryErr != nil {
			return "", retryErr
		}
		return retryResult, nil
	}
	return result, err
}

func (rp *RefreshingProvider) RerunPipeline(repo domain.Repository, id domain.PipelineID) error {
	err := rp.inner.RerunPipeline(repo, id)
	if err != nil && errors.Is(err, domain.ErrUnauthorized) {
		return rp.handleUnauthorized(func() error {
			return rp.inner.RerunPipeline(repo, id)
		})
	}
	return err
}

func (rp *RefreshingProvider) CancelPipeline(repo domain.Repository, id domain.PipelineID) error {
	err := rp.inner.CancelPipeline(repo, id)
	if err != nil && errors.Is(err, domain.ErrUnauthorized) {
		return rp.handleUnauthorized(func() error {
			return rp.inner.CancelPipeline(repo, id)
		})
	}
	return err
}
