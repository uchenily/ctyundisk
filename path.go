package main

import (
	"fmt"
	"path"
	"strings"
)

func cleanCloudPath(p string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(strings.TrimSpace(p), "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func defaultCloudPath() string {
	if config != nil && config.WorkDir != "" {
		return config.WorkDir
	}
	return "/"
}

func resolveCloudPath(p string) string {
	if strings.TrimSpace(p) == "" {
		return defaultCloudPath()
	}
	if strings.HasPrefix(p, "/") {
		return cleanCloudPath(p)
	}
	return cleanCloudPath(path.Join(defaultCloudPath(), p))
}

func pathToID(path string) (id string, isDir bool, size int64, err error) {
	path = resolveCloudPath(path)
	if path == "/" {
		return RootFolder, true, 0, nil
	}
	cleanPath := strings.TrimPrefix(path, "/")
	parts := strings.Split(cleanPath, "/")
	currentID := RootFolder
	for i, part := range parts {
		if part == "" {
			continue
		}
		isLast := i == len(parts)-1
		files, folders, fErr := listFilesPage(currentID, 1)
		if fErr != nil {
			return "", false, 0, fmt.Errorf("读取目录失败: %w", fErr)
		}
		if isLast {
			for _, f := range folders {
				if f.Name == part {
					return f.ID.String(), true, 0, nil
				}
			}
			for _, f := range files {
				if f.Name == part {
					return f.ID.String(), false, f.Size, nil
				}
			}
			return "", false, 0, fmt.Errorf("文件/目录未找到: %s", path)
		}
		found := false
		for _, f := range folders {
			if f.Name == part {
				currentID = f.ID.String()
				found = true
				break
			}
		}
		if !found {
			return "", false, 0, fmt.Errorf("目录未找到: /%s", strings.Join(parts[:i+1], "/"))
		}
	}
	return currentID, true, 0, nil
}
