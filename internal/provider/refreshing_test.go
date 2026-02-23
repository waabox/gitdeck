// internal/provider/refreshing_test.go
package provider_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/provider"
)

// mockProvider is a simple mock for testing the wrapper.
type mockProvider struct {
	listErr   error
	pipelines []domain.Pipeline
}

func (m *mockProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
	return m.pipelines, m.listErr
}
func (m *mockProvider) GetPipeline(_ domain.Repository, _ domain.PipelineID) (domain.Pipeline, error) {
	return domain.Pipeline{}, m.listErr
}
func (m *mockProvider) GetJobLogs(_ domain.Repository, _ domain.JobID) (string, error) {
	return "", m.listErr
}
func (m *mockProvider) RerunPipeline(_ domain.Repository, _ domain.PipelineID) error {
	return m.listErr
}
func (m *mockProvider) CancelPipeline(_ domain.Repository, _ domain.PipelineID) error {
	return m.listErr
}

func TestRefreshingProvider_PassesThroughOnSuccess(t *testing.T) {
	inner := &mockProvider{
		pipelines: []domain.Pipeline{{ID: "1"}},
	}
	rp := provider.NewRefreshingProvider(inner, "gitlab",
		func() (string, error) { return "", nil },
		func(token string) {},
	)

	result, err := rp.ListPipelines(domain.Repository{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 || result[0].ID != "1" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestRefreshingProvider_PassesThroughNon401Errors(t *testing.T) {
	inner := &mockProvider{
		listErr: fmt.Errorf("network timeout"),
	}
	rp := provider.NewRefreshingProvider(inner, "gitlab",
		func() (string, error) { return "", nil },
		func(token string) {},
	)

	_, err := rp.ListPipelines(domain.Repository{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "network timeout" {
		t.Errorf("expected 'network timeout', got: %v", err)
	}
}

func TestRefreshingProvider_RefreshesAndRetriesOn401(t *testing.T) {
	calls := 0
	inner := &failOnceProvider{
		firstErr:   fmt.Errorf("gitlab API error: 401 Unauthorized: %w", domain.ErrUnauthorized),
		secondResp: []domain.Pipeline{{ID: "refreshed"}},
	}

	refreshCalled := false
	tokenUpdated := ""
	rp := provider.NewRefreshingProvider(inner, "gitlab",
		func() (string, error) {
			refreshCalled = true
			return "new-token", nil
		},
		func(token string) {
			calls++
			tokenUpdated = token
		},
	)

	result, err := rp.ListPipelines(domain.Repository{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !refreshCalled {
		t.Error("expected refresh to be called")
	}
	if tokenUpdated != "new-token" {
		t.Errorf("expected token 'new-token', got '%s'", tokenUpdated)
	}
	if len(result) != 1 || result[0].ID != "refreshed" {
		t.Errorf("expected refreshed pipeline, got: %v", result)
	}
}

func TestRefreshingProvider_ReturnsAuthExpiredWhenRefreshFails(t *testing.T) {
	inner := &mockProvider{
		listErr: fmt.Errorf("gitlab API error: %w", domain.ErrUnauthorized),
	}
	rp := provider.NewRefreshingProvider(inner, "gitlab",
		func() (string, error) {
			return "", fmt.Errorf("refresh token revoked")
		},
		func(token string) {},
	)

	_, err := rp.ListPipelines(domain.Repository{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var authErr *provider.AuthExpiredError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthExpiredError, got: %T %v", err, err)
	}
	if authErr.Provider != "gitlab" {
		t.Errorf("expected provider 'gitlab', got '%s'", authErr.Provider)
	}
}

func TestRefreshingProvider_GetPipeline_RetriesOn401(t *testing.T) {
	inner := &failOncePipelineProvider{
		firstErr:   fmt.Errorf("gitlab API error: %w", domain.ErrUnauthorized),
		secondResp: domain.Pipeline{ID: "42"},
	}
	rp := provider.NewRefreshingProvider(inner, "gitlab",
		func() (string, error) { return "new-token", nil },
		func(token string) {},
	)

	result, err := rp.GetPipeline(domain.Repository{}, "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "42" {
		t.Errorf("expected pipeline ID '42', got '%s'", result.ID)
	}
}

func TestRefreshingProvider_RerunPipeline_RetriesOn401(t *testing.T) {
	calls := 0
	inner := &mockProvider{
		listErr: fmt.Errorf("gitlab API error: %w", domain.ErrUnauthorized),
	}
	// Override RerunPipeline behavior: first call fails, second succeeds
	rerunProvider := &failOnceRerunProvider{
		firstErr: fmt.Errorf("gitlab API error: %w", domain.ErrUnauthorized),
	}
	rp := provider.NewRefreshingProvider(rerunProvider, "gitlab",
		func() (string, error) {
			calls++
			return "new-token", nil
		},
		func(token string) {},
	)

	err := rp.RerunPipeline(domain.Repository{}, "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = inner // suppress unused warning
}

// Test helpers

// failOnceProvider returns an error on first ListPipelines call, success on second.
type failOnceProvider struct {
	calls      int
	firstErr   error
	secondResp []domain.Pipeline
}

func (f *failOnceProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
	f.calls++
	if f.calls == 1 {
		return nil, f.firstErr
	}
	return f.secondResp, nil
}
func (f *failOnceProvider) GetPipeline(_ domain.Repository, _ domain.PipelineID) (domain.Pipeline, error) {
	return domain.Pipeline{}, nil
}
func (f *failOnceProvider) GetJobLogs(_ domain.Repository, _ domain.JobID) (string, error) {
	return "", nil
}
func (f *failOnceProvider) RerunPipeline(_ domain.Repository, _ domain.PipelineID) error { return nil }
func (f *failOnceProvider) CancelPipeline(_ domain.Repository, _ domain.PipelineID) error {
	return nil
}

type failOncePipelineProvider struct {
	calls      int
	firstErr   error
	secondResp domain.Pipeline
}

func (f *failOncePipelineProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
	return nil, nil
}
func (f *failOncePipelineProvider) GetPipeline(_ domain.Repository, _ domain.PipelineID) (domain.Pipeline, error) {
	f.calls++
	if f.calls == 1 {
		return domain.Pipeline{}, f.firstErr
	}
	return f.secondResp, nil
}
func (f *failOncePipelineProvider) GetJobLogs(_ domain.Repository, _ domain.JobID) (string, error) {
	return "", nil
}
func (f *failOncePipelineProvider) RerunPipeline(_ domain.Repository, _ domain.PipelineID) error {
	return nil
}
func (f *failOncePipelineProvider) CancelPipeline(_ domain.Repository, _ domain.PipelineID) error {
	return nil
}

type failOnceRerunProvider struct {
	calls    int
	firstErr error
}

func (f *failOnceRerunProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
	return nil, nil
}
func (f *failOnceRerunProvider) GetPipeline(_ domain.Repository, _ domain.PipelineID) (domain.Pipeline, error) {
	return domain.Pipeline{}, nil
}
func (f *failOnceRerunProvider) GetJobLogs(_ domain.Repository, _ domain.JobID) (string, error) {
	return "", nil
}
func (f *failOnceRerunProvider) RerunPipeline(_ domain.Repository, _ domain.PipelineID) error {
	f.calls++
	if f.calls == 1 {
		return f.firstErr
	}
	return nil
}
func (f *failOnceRerunProvider) CancelPipeline(_ domain.Repository, _ domain.PipelineID) error {
	return nil
}
