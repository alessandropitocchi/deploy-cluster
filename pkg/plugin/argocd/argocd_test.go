package argocd

import (
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

func testLogger() *logger.Logger {
	return logger.New("[argocd]", logger.LevelQuiet)
}

func TestNew(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	if p == nil {
		t.Fatal("New() should return non-nil plugin")
	}
	if p.Log == nil {
		t.Error("Log should not be nil")
	}
	if p.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", p.Timeout, 5*time.Minute)
	}
}

func TestNew_CustomTimeout(t *testing.T) {
	p := New(testLogger(), 30*time.Second)
	if p.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", p.Timeout, 30*time.Second)
	}
}

func TestRepoName_ExplicitName(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	repo := template.ArgoCDRepoTemplate{
		Name: "my-repo",
		URL:  "https://github.com/user/repo.git",
	}
	got := p.repoName(repo)
	if got != "my-repo" {
		t.Errorf("repoName() = %q, want %q", got, "my-repo")
	}
}

func TestRepoName_GeneratedFromHTTPS(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	repo := template.ArgoCDRepoTemplate{
		URL: "https://github.com/user/repo.git",
	}
	got := p.repoName(repo)
	want := "github-com-user-repo"
	if got != want {
		t.Errorf("repoName() = %q, want %q", got, want)
	}
}

func TestRepoName_GeneratedFromSSH(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	repo := template.ArgoCDRepoTemplate{
		URL: "git@github.com:user/repo.git",
	}
	got := p.repoName(repo)
	want := "github-com-user-repo"
	if got != want {
		t.Errorf("repoName() = %q, want %q", got, want)
	}
}

func TestRepoName_TrimsGitSuffix(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)
	repo := template.ArgoCDRepoTemplate{
		URL: "https://github.com/user/myapp.git",
	}
	got := p.repoName(repo)
	// ".git" becomes "-git", then TrimSuffix removes it
	want := "github-com-user-myapp"
	if got != want {
		t.Errorf("repoName() = %q, want %q", got, want)
	}
}

func TestDiffRepos_AddAndRemove(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)

	desiredRepos := []template.ArgoCDRepoTemplate{
		{Name: "repo-a", URL: "https://a.com"},
		{Name: "repo-b", URL: "https://b.com"},
	}
	currentRepoNames := []string{"repo-a", "repo-c"}

	// Build desired map
	desiredMap := make(map[string]template.ArgoCDRepoTemplate)
	for _, repo := range desiredRepos {
		desiredMap[p.repoName(repo)] = repo
	}

	// Repos to remove: in current but not in desired
	var toRemove []string
	for _, name := range currentRepoNames {
		if _, ok := desiredMap[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}

	if len(toRemove) != 1 || toRemove[0] != "repo-c" {
		t.Errorf("toRemove = %v, want [repo-c]", toRemove)
	}
}

func TestDiffApps_AddAndRemove(t *testing.T) {
	desiredApps := []template.ArgoCDAppTemplate{
		{Name: "app-x"},
		{Name: "app-y"},
	}
	currentAppNames := []string{"app-x", "app-z"}

	desiredMap := make(map[string]template.ArgoCDAppTemplate)
	for _, app := range desiredApps {
		desiredMap[app.Name] = app
	}

	var toRemove []string
	for _, name := range currentAppNames {
		if _, ok := desiredMap[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}

	if len(toRemove) != 1 || toRemove[0] != "app-z" {
		t.Errorf("toRemove = %v, want [app-z]", toRemove)
	}
}

func TestDiffRepos_AllNew(t *testing.T) {
	p := New(testLogger(), 5*time.Minute)

	desiredRepos := []template.ArgoCDRepoTemplate{
		{Name: "repo-a", URL: "https://a.com"},
	}
	var currentRepoNames []string

	desiredMap := make(map[string]template.ArgoCDRepoTemplate)
	for _, repo := range desiredRepos {
		desiredMap[p.repoName(repo)] = repo
	}

	var toRemove []string
	for _, name := range currentRepoNames {
		if _, ok := desiredMap[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}

	if len(toRemove) != 0 {
		t.Errorf("toRemove = %v, want empty", toRemove)
	}
}

func TestDiffApps_AllRemoved(t *testing.T) {
	var desiredApps []template.ArgoCDAppTemplate
	currentAppNames := []string{"app-old1", "app-old2"}

	desiredMap := make(map[string]template.ArgoCDAppTemplate)
	for _, app := range desiredApps {
		desiredMap[app.Name] = app
	}

	var toRemove []string
	for _, name := range currentAppNames {
		if _, ok := desiredMap[name]; !ok {
			toRemove = append(toRemove, name)
		}
	}

	if len(toRemove) != 2 {
		t.Errorf("toRemove = %v, want [app-old1 app-old2]", toRemove)
	}
}
