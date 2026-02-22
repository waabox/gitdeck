package provider_test

import (
	"testing"

	"github.com/waabox/gitdeck/internal/domain"
	"github.com/waabox/gitdeck/internal/provider"
)

type fakeProvider struct{ name string }

func (f *fakeProvider) ListPipelines(_ domain.Repository) ([]domain.Pipeline, error) {
	return nil, nil
}
func (f *fakeProvider) GetPipeline(_ domain.Repository, _ domain.PipelineID) (domain.Pipeline, error) {
	return domain.Pipeline{}, nil
}

func TestRegistry_DetectsGitHub(t *testing.T) {
	gh := &fakeProvider{name: "github"}
	gl := &fakeProvider{name: "gitlab"}

	reg := provider.NewRegistry()
	reg.Register("github.com", gh)
	reg.Register("gitlab.com", gl)

	p, err := reg.Detect("https://github.com/waabox/gitdeck.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != gh {
		t.Error("expected github provider to be detected")
	}
}

func TestRegistry_DetectsGitLab(t *testing.T) {
	gh := &fakeProvider{name: "github"}
	gl := &fakeProvider{name: "gitlab"}

	reg := provider.NewRegistry()
	reg.Register("github.com", gh)
	reg.Register("gitlab.com", gl)

	p, err := reg.Detect("https://gitlab.com/mygroup/myproject.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != gl {
		t.Error("expected gitlab provider to be detected")
	}
}

func TestRegistry_DetectsSelfHostedGitLab(t *testing.T) {
	gl := &fakeProvider{name: "gitlab-self"}

	reg := provider.NewRegistry()
	reg.Register("gitlab.mycompany.com", gl)

	p, err := reg.Detect("https://gitlab.mycompany.com/team/project.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != gl {
		t.Error("expected self-hosted gitlab provider to be detected")
	}
}

func TestRegistry_ErrorOnUnknownHost(t *testing.T) {
	reg := provider.NewRegistry()

	_, err := reg.Detect("https://bitbucket.org/user/repo.git")
	if err == nil {
		t.Fatal("expected error for unknown host, got nil")
	}
}
