package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestState_LoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()

	s := &GitteState{
		Toggles: map[string]bool{"proj-a": true, "proj-b": false},
		Features: map[string]FeatureState{
			"my-feature": {Enabled: true},
		},
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Toggles["proj-a"] != true || loaded.Toggles["proj-b"] != false {
		t.Errorf("toggle state not preserved: %v", loaded.Toggles)
	}
	if !loaded.Features["my-feature"].Enabled {
		t.Error("feature state not preserved")
	}
	if loaded.Version != StateVersion {
		t.Errorf("version not set: got %d, want %d", loaded.Version, StateVersion)
	}
}

func TestState_LoadMissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load of missing file should not error: %v", err)
	}
	if s.Toggles == nil {
		t.Error("Toggles map should be initialized, not nil")
	}
	if s.Features == nil {
		t.Error("Features map should be initialized, not nil")
	}
}

func TestState_LoadMissingFileNilMapAccess(t *testing.T) {
	dir := t.TempDir()

	s, _ := Load(dir)
	// These should not panic even though no state file exists
	_ = s.Toggles["any-key"]
	_ = s.Features["any-key"]
}

func TestState_EnsureGitignored_AddsEntry(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureGitignored(dir); err != nil {
		t.Fatalf("EnsureGitignored failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore to exist: %v", err)
	}
	content := string(data)
	if !containsLine(content, StateFileName) {
		t.Errorf(".gitignore does not contain %q", StateFileName)
	}
}

func TestState_EnsureGitignored_IdempotentOnSecondCall(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureGitignored(dir); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if err := EnsureGitignored(dir); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	count := 0
	for _, line := range splitLines(string(data)) {
		if line == StateFileName {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of %q in .gitignore, got %d", StateFileName, count)
	}
}

func TestState_SaveSetsVersion(t *testing.T) {
	dir := t.TempDir()

	s := &GitteState{
		Version: 0, // should be overwritten by Save
		Toggles: make(map[string]bool),
		Features: make(map[string]FeatureState),
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.Version != StateVersion {
		t.Errorf("Save should set Version to %d, got %d", StateVersion, loaded.Version)
	}
}

func TestState_RemoteConfigCacheRoundtrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	s := &GitteState{
		Toggles:  make(map[string]bool),
		Features: make(map[string]FeatureState),
		Cache: StateCache{
			RemoteConfig: &RemoteConfigCacheEntry{
				RemoteGitRepo: "git@host:org/config.git",
				RemoteGitRef:  "main",
				RemoteGitFile: "gitte.yml",
				FetchedAt:     now,
				Content:       "projects: {}",
			},
		},
	}

	if err := Save(dir, s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Cache.RemoteConfig == nil {
		t.Fatal("expected RemoteConfig cache to be preserved")
	}
	c := loaded.Cache.RemoteConfig
	if c.RemoteGitRepo != "git@host:org/config.git" || c.Content != "projects: {}" {
		t.Errorf("cache content not preserved: %+v", c)
	}
	if !c.FetchedAt.Equal(now) {
		t.Errorf("FetchedAt not preserved: got %v, want %v", c.FetchedAt, now)
	}
}
