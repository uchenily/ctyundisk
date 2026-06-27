package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func cmdDownload(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: yd download <文件路径>")
		os.Exit(1)
	}
	filePath := args[0]
	if err := doDownload(filePath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doDownload(path string) error {
	fileID, isDir, fileSize, err := pathToID(path)
	if err != nil {
		return err
	}
	if isDir {
		return fmt.Errorf("路径指向的是目录而非文件: %s", path)
	}

	name := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		name = path[idx+1:]
	}

	fmt.Printf("下载 %s (%d 字节)...\n", path, fileSize)

	downloadURL, err := getDownloadURLByID(fileID)
	if err != nil {
		return err
	}

	return downloadFile(downloadURL, name, fileSize)
}

func downloadFile(downloadURL, localName string, totalSize int64) error {
	var offset int64

	info, err := os.Stat(localName)
	if err == nil {
		offset = info.Size()
		if offset >= totalSize {
			fmt.Println("文件已完整下载")
			return nil
		}
	}

	file, err := os.OpenFile(localName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	progress := newProgressBar("下载", totalSize)
	progress.uploaded = offset
	if offset > 0 {
		progress.lastRender = time.Now()
		progress.render(false)
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, totalSize))
		fmt.Printf("从断点 %d 续传...\n", offset)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("下载失败 HTTP %d: %s", resp.StatusCode, string(body))
	}

	_, err = io.Copy(file, progress.reader(resp.Body))
	if err != nil {
		return err
	}
	progress.finish()
	fmt.Printf("下载完成: %s\n", localName)
	return nil
}
