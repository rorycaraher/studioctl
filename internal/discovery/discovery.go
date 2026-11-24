// Package discovery finds Projects on disk and tells which DAW made them,
// using structural conventions (a DAW's project file at the top level of a
// folder) rather than a naming exclude-list.
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// DAW is the program a Project belongs to.
type DAW string

const (
	// Unknown means no supported DAW's project layout was recognised.
	Unknown DAW = ""
	Ableton DAW = "Ableton Live"
)

// Supported lists the DAWs discovery can recognise, for error messages.
const Supported = "Ableton Live"

// Project is a folder holding one DAW's project: one or more top-level
// Versions.
type Project struct {
	// Name is the folder name, without the " Project" suffix Live appends
	// to project folders.
	Name string
	Path string
	DAW  DAW
	// Versions are absolute paths to the project's top-level project files
	// (for Ableton, .als files; Live's auto-generated Backup/ subfolder is
	// not scanned).
	Versions []string
}

const folderSuffix = " Project"

// ProjectName strips the " Project" suffix Live appends to project folder
// names. A folder named exactly " Project" is left as is.
func ProjectName(folder string) string {
	if n := strings.TrimSuffix(folder, folderSuffix); n != "" {
		return n
	}
	return folder
}

// SamplesDir returns the project's Samples directory path, which may not exist.
func (p Project) SamplesDir() string {
	return filepath.Join(p.Path, "Samples")
}

// detector recognises one DAW's Projects. versions returns a folder's
// top-level project files; a folder with none isn't that DAW's Project.
type detector struct {
	daw      DAW
	versions func(dir string) ([]string, error)
	// versionExt is the extension of a single Version file.
	versionExt string
	// generated names the folders the DAW creates inside its Projects
	// (e.g. Live's Backup/). DiscoverProjects never searches them, so a
	// Project whose Versions are all gone isn't mistaken for a grouping
	// folder full of Projects.
	generated []string
}

var detectors = []detector{
	{
		daw:        Ableton,
		versions:   topLevelAlsFiles,
		versionExt: ".als",
		generated:  []string{"Backup", "Ableton Project Info"},
	},
}

// isGenerated reports whether name is a folder some DAW creates inside its
// Projects.
func isGenerated(name string) bool {
	for _, d := range detectors {
		if slices.Contains(d.generated, name) {
			return true
		}
	}
	return false
}

// LoadProject reads the folder at path as a Project, recognising its DAW.
// It returns ok=false, with no error, when no supported DAW's layout is
// there.
func LoadProject(path string) (p Project, ok bool, err error) {
	for _, d := range detectors {
		versions, err := d.versions(path)
		if err != nil {
			return Project{}, false, err
		}
		if len(versions) > 0 {
			return Project{
				Name:     ProjectName(filepath.Base(path)),
				Path:     path,
				DAW:      d.daw,
				Versions: versions,
			}, true, nil
		}
	}
	return Project{}, false, nil
}

// VersionDAW returns the DAW whose project file path is, by extension, or
// Unknown.
func VersionDAW(path string) DAW {
	for _, d := range detectors {
		if strings.EqualFold(filepath.Ext(path), d.versionExt) {
			return d.daw
		}
	}
	return Unknown
}

// DiscoverProjects finds Projects anywhere under dir: any directory with a
// supported DAW's top-level project file, however deeply it's grouped into
// subfolders. A Project's own subfolders (Samples/, Backup/, ...) are never
// searched for further Projects. Folders without a project file (reference
// material, sample packs, grouping folders) are searched but not listed;
// hidden folders are skipped, and dir itself is never a Project.
func DiscoverProjects(dir string) ([]Project, error) {
	var projects []Project
	if err := findProjects(dir, &projects); err != nil {
		return nil, err
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].Name != projects[j].Name {
			return projects[i].Name < projects[j].Name
		}
		return projects[i].Path < projects[j].Path
	})
	return projects, nil
}

// findProjects appends the Projects among dir's subdirectories, recursing
// into those that aren't Projects. Symlinked directories aren't followed.
func findProjects(dir string, projects *[]Project) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || isGenerated(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		p, ok, err := LoadProject(path)
		if err != nil {
			return err
		}
		if ok {
			*projects = append(*projects, p)
			continue
		}
		if err := findProjects(path, projects); err != nil {
			return err
		}
	}
	return nil
}

// DetectDAW returns the DAW of the Projects under dir, judged by
// the first Project found: every Project in a Projects directory is assumed
// to come from the same DAW. It returns Unknown if there are none.
func DetectDAW(dir string) (DAW, error) {
	projects, err := DiscoverProjects(dir)
	if err != nil || len(projects) == 0 {
		return Unknown, err
	}
	return projects[0].DAW, nil
}

func topLevelAlsFiles(projectPath string) ([]string, error) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", projectPath, err)
	}
	var als []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".als") {
			als = append(als, filepath.Join(projectPath, e.Name()))
		}
	}
	sort.Strings(als)
	return als, nil
}
