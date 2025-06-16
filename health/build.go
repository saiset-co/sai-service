package health

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type BuildInfo struct {
	Version   string    `json:"version"`
	GitCommit string    `json:"git_commit"`
	GitBranch string    `json:"git_branch"`
	BuildTime time.Time `json:"build_time"`
	GoVersion string    `json:"go_version"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	Compiler  string    `json:"compiler"`
}

func getBuildInfo() string {
	buildInfo := &BuildInfo{
		Version:   getEnvOrDefault("BUILD_VERSION", "dev"),
		GitCommit: getEnvOrDefault("BUILD_COMMIT", "unknown"),
		GitBranch: getEnvOrDefault("BUILD_BRANCH", "unknown"),
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Compiler:  runtime.Compiler,
	}

	if buildTimeStr := getEnvOrDefault("BUILD_TIME", ""); buildTimeStr != "" {
		if buildTime, err := time.Parse(time.RFC3339, buildTimeStr); err == nil {
			buildInfo.BuildTime = buildTime
		}
	}

	if fileInfo := readBuildInfoFile(); fileInfo != nil {
		if fileInfo.Version != "" {
			buildInfo.Version = fileInfo.Version
		}
		if fileInfo.GitCommit != "" {
			buildInfo.GitCommit = fileInfo.GitCommit
		}
		if fileInfo.GitBranch != "" {
			buildInfo.GitBranch = fileInfo.GitBranch
		}
		if !fileInfo.BuildTime.IsZero() {
			buildInfo.BuildTime = fileInfo.BuildTime
		}
	}

	return fmt.Sprintf("%s-%s (%s)", buildInfo.Version, buildInfo.GitCommit[:_min(len(buildInfo.GitCommit), 7)], buildInfo.BuildTime.Format("2006-01-02"))
}

func readBuildInfoFile() *BuildInfo {
	paths := []string{
		"build.info",
		"./build.info",
		"../build.info",
		"/app/build.info",
	}

	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			return parseBuildInfoFile(string(data))
		}
	}

	return nil
}

func parseBuildInfoFile(content string) *BuildInfo {
	buildInfo := &BuildInfo{}
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "VERSION":
			buildInfo.Version = value
		case "GIT_COMMIT":
			buildInfo.GitCommit = value
		case "GIT_BRANCH":
			buildInfo.GitBranch = value
		case "BUILD_TIME":
			if buildTime, err := time.Parse(time.RFC3339, value); err == nil {
				buildInfo.BuildTime = buildTime
			}
		}
	}

	return buildInfo
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func _min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
