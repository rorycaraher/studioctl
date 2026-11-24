package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustTouch(t *testing.T, path string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverProjects_DetectsViaTopLevelAls(t *testing.T) {
	prod := t.TempDir()

	// Real project: has a top-level .als.
	mustTouch(t, filepath.Join(prod, "Track One", "Track One.als"))
	// Non-project folder: no .als anywhere at the top level.
	mustMkdir(t, filepath.Join(prod, "Reference Material"))
	mustTouch(t, filepath.Join(prod, "Reference Material", "notes.txt"))
	// .als exists but only nested in a Backup folder, not top-level -> not a project.
	mustTouch(t, filepath.Join(prod, "Half Baked", "Backup", "Half Baked [2024-01-01 120000].als"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1: %+v", len(projects), projects)
	}
	if projects[0].Name != "Track One" {
		t.Fatalf("got project %q, want Track One", projects[0].Name)
	}
	if len(projects[0].Versions) != 1 {
		t.Fatalf("got %d als files, want 1", len(projects[0].Versions))
	}
}

func TestDiscoverProjects_IgnoresBackupFolderAls(t *testing.T) {
	prod := t.TempDir()
	mustTouch(t, filepath.Join(prod, "Song", "Song.als"))
	mustTouch(t, filepath.Join(prod, "Song", "Backup", "Song [old].als"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || len(projects[0].Versions) != 1 {
		t.Fatalf("expected exactly one top-level als, got %+v", projects)
	}
}

func TestProjectName(t *testing.T) {
	if got := ProjectName("installations Project"); got != "installations" {
		t.Errorf("ProjectName = %q", got)
	}
	if got := ProjectName("Track One"); got != "Track One" {
		t.Errorf("ProjectName = %q", got)
	}
}

func TestLoadProject(t *testing.T) {
	prod := t.TempDir()
	mustTouch(t, filepath.Join(prod, "Song Project", "Song.als"))
	mustTouch(t, filepath.Join(prod, "Notes", "notes.txt"))

	p, ok, err := LoadProject(filepath.Join(prod, "Song Project"))
	if err != nil || !ok {
		t.Fatalf("LoadProject: ok=%v err=%v", ok, err)
	}
	if p.DAW != Ableton || p.Name != "Song" || len(p.Versions) != 1 {
		t.Errorf("got %+v", p)
	}

	if _, ok, err := LoadProject(filepath.Join(prod, "Notes")); ok || err != nil {
		t.Errorf("Notes: ok=%v err=%v, want not a project", ok, err)
	}
	if _, _, err := LoadProject(filepath.Join(prod, "missing")); err == nil {
		t.Error("missing folder: want error")
	}
}

func TestDetectDAW(t *testing.T) {
	prod := t.TempDir()
	mustTouch(t, filepath.Join(prod, "Reference", "notes.txt"))
	if got, err := DetectDAW(prod); err != nil || got != Unknown {
		t.Errorf("no projects: got %q, %v; want Unknown", got, err)
	}

	mustTouch(t, filepath.Join(prod, "Song", "Song.als"))
	if got, err := DetectDAW(prod); err != nil || got != Ableton {
		t.Errorf("got %q, %v; want Ableton", got, err)
	}

	if _, err := DetectDAW(filepath.Join(prod, "missing")); err == nil {
		t.Error("missing dir: want error")
	}
}

func TestVersionDAW(t *testing.T) {
	for path, want := range map[string]DAW{
		"/p/Song.als":    Ableton,
		"/p/Song.ALS":    Ableton,
		"/p/Song.wav":    Unknown,
		"/p/Song":        Unknown,
		"/p/Song.logicx": Unknown,
	} {
		if got := VersionDAW(path); got != want {
			t.Errorf("VersionDAW(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDiscoverProjects_FindsNestedProjects(t *testing.T) {
	prod := t.TempDir()
	mustTouch(t, filepath.Join(prod, "Top", "Top.als"))
	mustTouch(t, filepath.Join(prod, "2024", "Album", "Deep Project", "Deep.als"))
	mustTouch(t, filepath.Join(prod, "2024", "Loose Project", "Loose.als"))
	// Inside a Project: not searched further, so no nested Project here.
	mustTouch(t, filepath.Join(prod, "Top", "Stems", "Inner", "Inner.als"))
	// Hidden folders are skipped.
	mustTouch(t, filepath.Join(prod, ".Trash", "Gone", "Gone.als"))
	// A grouping folder with no Projects anywhere below.
	mustTouch(t, filepath.Join(prod, "Reference", "Sub", "notes.txt"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, p := range projects {
		got[p.Name] = p.Path
	}
	want := map[string]string{
		"Top":   filepath.Join(prod, "Top"),
		"Deep":  filepath.Join(prod, "2024", "Album", "Deep Project"),
		"Loose": filepath.Join(prod, "2024", "Loose Project"),
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for name, path := range want {
		if got[name] != path {
			t.Errorf("%s: got path %q, want %q", name, got[name], path)
		}
	}
}

func TestDiscoverProjects_RootIsNotAProject(t *testing.T) {
	prod := t.TempDir()
	mustTouch(t, filepath.Join(prod, "stray.als"))
	mustTouch(t, filepath.Join(prod, "Song", "Song.als"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "Song" {
		t.Fatalf("got %+v, want just Song", projects)
	}
}

func TestDiscoverProjects_SkipsGeneratedFoldersWhenRecursing(t *testing.T) {
	prod := t.TempDir()
	// Versions deleted, only Live's own folders left: not a Project, and
	// nothing inside counts as one either.
	mustTouch(t, filepath.Join(prod, "Group", "Dead Project", "Backup", "Dead [2024-01-01 120000].als"))
	mustTouch(t, filepath.Join(prod, "Group", "Dead Project", "Ableton Project Info", "x.als"))
	mustTouch(t, filepath.Join(prod, "Group", "Live Project", "Live.als"))

	projects, err := DiscoverProjects(prod)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "Live" {
		t.Fatalf("got %+v, want just Live", projects)
	}
}
