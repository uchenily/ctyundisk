package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func cmdList(args []string) {
	path := ""
	if len(args) >= 1 {
		path = args[0]
	}
	folderID, isDir, _, err := pathToID(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !isDir {
		fmt.Fprintln(os.Stderr, "路径指向的是文件而非目录:", path)
		os.Exit(1)
	}
	if err := doList(folderID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func displayWidth(s string) int {
	width := 0
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0x20000 && r <= 0x2A6DF) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0xFF00 && r <= 0xFFEF) {
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

func doList(folderID string) error {
	files, folders, err := listFilesPage(folderID, 1)
	if err != nil {
		return err
	}

	type Item struct {
		Name string
		Info string
	}
	var items []Item

	for _, f := range folders {
		items = append(items, Item{Name: f.Name, Info: "[DIR]"})
	}
	for _, f := range files {
		items = append(items, Item{Name: f.Name, Info: formatSize(f.Size)})
	}

	maxNameWidth := 0
	for _, it := range items {
		w := displayWidth(it.Name)
		if w > maxNameWidth {
			maxNameWidth = w
		}
	}

	for _, it := range items {
		nameWidth := displayWidth(it.Name)
		padding := maxNameWidth - nameWidth
		fmt.Printf("%s%s %s\n", it.Name, strings.Repeat(" ", padding), it.Info)
	}
	return nil
}

func listFilesPage(folderID string, page int) (files, folders []cloudFile, err error) {
	req, err := apiGet("/listFiles.action", url.Values{
		"folderId":   {folderID},
		"fileType":   {"0"},
		"mediaType":  {"0"},
		"mediaAttr":  {"0"},
		"iconOption": {"0"},
		"orderBy":    {"filename"},
		"descending": {"true"},
		"pageNum":    {strconv.Itoa(page)},
		"pageSize":   {"100"},
	})
	if err != nil {
		return nil, nil, err
	}

	var resp listResp
	if err := doAPI(req, &resp); err != nil {
		return nil, nil, err
	}

	files = resp.FileListAO.FileList
	folders = resp.FileListAO.FolderList
	for i := range folders {
		folders[i].IsDir = true
	}

	if 100*page < resp.FileListAO.Count {
		moreFiles, moreFolders, err := listFilesPage(folderID, page+1)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, moreFiles...)
		folders = append(folders, moreFolders...)
	}

	return files, folders, nil
}

func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
