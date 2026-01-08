package internal

import (
	"gitte/config"
	"os"
	"path/filepath"
	"strings"
)

const toggleFileName = ".gitte-projects-toggled"

func SaveToggledProjects(cwd string, toggledProjects config.ToggledProjects) error {
	var lines []string
	for project, toggled := range toggledProjects {
		line := project + ":"
		if toggled {
			line += "true"
		} else {
			line += "false"
		}
		lines = append(lines, line)
	}
	content := strings.Join(lines, "\n")
	filePath := getToggleFilePath(cwd)
	return os.WriteFile(filePath, []byte(content), 0644)
}

func getToggleFilePath(cwd string) string {
	return filepath.Join(cwd, toggleFileName)
}

func ReadToggledProjects(cwd string) (config.ToggledProjects, error) {

	filePath := getToggleFilePath(cwd)
	return parseToggleFile(filePath)
}

func parseToggleFile(filePath string) (config.ToggledProjects, error) {
	content, err := os.ReadFile(filePath)

	if err != nil {
		if os.IsNotExist(err) {
			return config.ToggledProjects{}, nil
		}
		return nil, err
	}

	return parseToggleFileContent(content)
}

func parseToggleFileContent(content []byte) (config.ToggledProjects, error) {
	toggledProjects := make(config.ToggledProjects)
	// The file is parsed a each newline being project:(true|false)
	// for example my-project:true
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		projectKey := parts[0]
		toggleValue := parts[1]
		if toggleValue == "true" {
			toggledProjects[projectKey] = true
		} else if toggleValue == "false" {
			toggledProjects[projectKey] = false
		}
	}
	return toggledProjects, nil
}
