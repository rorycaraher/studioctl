package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/rorycaraher/studioctl/internal/config"
	"github.com/rorycaraher/studioctl/internal/discovery"
)

func newProjectsCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List Projects in the projects Directory, with their Version counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(configPath)
			if err != nil {
				return err
			}
			dir, err := requireDir(cfg, config.Projects)
			if err != nil {
				return err
			}

			projects, err := discovery.DiscoverProjects(dir.Path)
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				fmt.Println(boldStyle.Render(dir.Path))
				fmt.Println(dimStyle.Render("  no Projects found (supported: " + discovery.Supported + ")"))
				return nil
			}
			// Every Project in the Directory is assumed to share a DAW.
			fmt.Println(boldStyle.Render(dir.Path) + "  " + dimStyle.Render(string(projects[0].DAW)))
			for _, line := range projectLines(dir.Path, projects) {
				fmt.Println(line)
			}
			return nil
		},
	}
	addConfigFlag(cmd, &configPath)
	return cmd
}

// projectLines renders the Project rows as aligned columns: names padded to
// the widest, counts right-aligned so the numbers line up. A Project in a
// subfolder of root is prefixed with that subfolder's path, and rows are
// sorted by that label so grouped Projects sit together.
func projectLines(root string, projects []discovery.Project) []string {
	projects = slices.Clone(projects)
	slices.SortStableFunc(projects, func(a, b discovery.Project) int {
		return strings.Compare(projectLabel(root, a), projectLabel(root, b))
	})
	labels := make([]string, len(projects))
	var nameW, countW int
	for i, p := range projects {
		labels[i] = projectLabel(root, p)
		nameW = max(nameW, lipgloss.Width(labels[i]))
		countW = max(countW, len(strconv.Itoa(len(p.Versions))))
	}

	lines := make([]string, 0, len(projects))
	for i, p := range projects {
		counts := fmt.Sprintf("%*d %s", countW, len(p.Versions), plural(len(p.Versions), "version"))
		pad := strings.Repeat(" ", nameW-lipgloss.Width(labels[i]))
		lines = append(lines, fmt.Sprintf("  - %s%s  %s", labels[i], pad, dimStyle.Render(counts)))
	}
	return lines
}

// projectLabel is p's name, prefixed with the path of the subfolder of root
// it's grouped in, if any: "2024/Album/Song".
func projectLabel(root string, p discovery.Project) string {
	group, err := filepath.Rel(root, filepath.Dir(p.Path))
	if err != nil || group == "." {
		return p.Name
	}
	return filepath.ToSlash(group) + "/" + p.Name
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
