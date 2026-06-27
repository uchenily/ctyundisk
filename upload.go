package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func cmdUpload(args []string) {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(os.Stderr, "用法: yd upload <本地文件路径> [目标目录]")
		os.Exit(1)
	}

	localPath := args[0]
	parentPath := ""
	if len(args) == 2 {
		parentPath = args[1]
	}

	parentID, isDir, _, err := pathToID(parentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "目标目录无效:", err)
		os.Exit(1)
	}
	if !isDir {
		fmt.Fprintln(os.Stderr, "目标路径不是目录:", parentPath)
		os.Exit(1)
	}

	if err := doUpload(localPath, parentID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doUpload(localPath, parentID string) error {
	req, _ := apiGet("/keepUserSession.action", nil)
	doAPI(req, nil)

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("不支持上传目录")
	}

	fileSize := info.Size()
	fileName := info.Name()
	sliceNum := int(math.Ceil(float64(fileSize) / float64(SliceSize)))

	fmt.Printf("上传 %s (%d 字节, %d 分片)...\n", fileName, fileSize, sliceNum)

	fileMD5, sliceMD5, partNames := calcFileMD5(f, fileSize, sliceNum)

	params := make(url.Values)
	params.Set("parentFolderId", parentID)
	params.Set("fileName", fileName)
	params.Set("fileSize", strconv.FormatInt(fileSize, 10))
	params.Set("sliceSize", strconv.Itoa(SliceSize))
	params.Set("fileMd5", fileMD5)
	params.Set("sliceMd5", sliceMD5)
	params.Set("extend", `{"opScene":"1","relativepath":"","rootfolderid":""}`)

	var initResp struct {
		Code string `json:"code"`
		Data struct {
			UploadFileId   string `json:"uploadFileId"`
			FileDataExists int    `json:"fileDataExists"`
		} `json:"data"`
	}

	req, err = buildUploadRequest("/person/initMultiUpload", params)
	if err != nil {
		return err
	}
	if err := doUploadAPI(req, &initResp); err != nil {
		return fmt.Errorf("初始化上传失败: %w", err)
	}

	uploadFileID := initResp.Data.UploadFileId
	if uploadFileID == "" {
		return fmt.Errorf("获取 uploadFileId 失败")
	}

	if initResp.Data.FileDataExists != 1 {
		partInfoParts := make([]string, sliceNum)
		for i := 0; i < sliceNum; i++ {
			partInfoParts[i] = fmt.Sprintf("%d-%s", i+1, partNames[i])
		}

		urlParams := make(url.Values)
		urlParams.Set("partInfo", strings.Join(partInfoParts, ","))
		urlParams.Set("uploadFileId", uploadFileID)

		var urlResp struct {
			Code string `json:"code"`
			Data map[string]struct {
				RequestURL    string `json:"requestURL"`
				RequestHeader string `json:"requestHeader"`
			} `json:"uploadUrls"`
		}

		req, err = buildUploadRequest("/person/getMultiUploadUrls", urlParams)
		if err != nil {
			return err
		}
		if err := doUploadAPI(req, &urlResp); err != nil {
			return fmt.Errorf("获取上传URL失败: %w", err)
		}

		_, _ = f.Seek(0, 0)
		progress := newProgressBar("上传", fileSize)
		for i := 0; i < sliceNum; i++ {
			num := strconv.Itoa(i + 1)
			partInfo := urlResp.Data["partNumber_"+num]
			if partInfo.RequestURL == "" {
				return fmt.Errorf("分片 %s 上传URL为空", num)
			}

			offset := int64(i) * SliceSize
			size := int64(SliceSize)
			if offset+size > fileSize {
				size = fileSize - offset
			}
			section := io.NewSectionReader(f, offset, size)

			putReq, err := http.NewRequest("PUT", partInfo.RequestURL, progress.reader(section))
			if err != nil {
				return err
			}
			for _, hdr := range strings.Split(partInfo.RequestHeader, "&") {
				if idx := strings.Index(hdr, "="); idx > 0 {
					putReq.Header.Set(hdr[:idx], hdr[idx+1:])
				}
			}

			putResp, err := http.DefaultClient.Do(putReq)
			if err != nil {
				return fmt.Errorf("上传分片 %d 失败: %w", i+1, err)
			}
			putResp.Body.Close()
			if putResp.StatusCode != 200 {
				return fmt.Errorf("上传分片 %d HTTP %d", i+1, putResp.StatusCode)
			}
		}
		progress.finish()
	} else {
		fmt.Println("秒传成功！")
	}

	commitParams := make(url.Values)
	commitParams.Set("uploadFileId", uploadFileID)
	commitParams.Set("opertype", "3")
	if initResp.Data.FileDataExists == 1 {
		commitParams.Set("lazyCheck", "0")
	} else {
		commitParams.Set("fileMd5", fileMD5)
		commitParams.Set("sliceMd5", sliceMD5)
		commitParams.Set("lazyCheck", "1")
	}

	var commitResp struct {
		Code string `json:"code"`
		File struct {
			Id string `json:"userFileId"`
		} `json:"file"`
	}
	req, err = buildUploadRequest("/person/commitMultiUploadFile", commitParams)
	if err != nil {
		return err
	}
	if err := doUploadAPI(req, &commitResp); err != nil {
		return fmt.Errorf("提交上传失败: %w", err)
	}

	dlURL, err := getDownloadURLByID(commitResp.File.Id)
	if err != nil {
		dlURL = "(获取下载链接失败)"
	}
	fmt.Printf("上传完成: %s\n%s\n", fileName, formatDownloadCommand(dlURL, fileName))
	return nil
}

func calcFileMD5(f *os.File, fileSize int64, sliceNum int) (fileMD5, sliceMD5 string, partNames []string) {
	_, _ = f.Seek(0, 0)

	global := md5.New()
	detail := md5.New()
	writer := io.MultiWriter(global, detail)
	slices := make([]string, sliceNum)
	partNames = make([]string, sliceNum)
	buf := make([]byte, 32*1024)

	for i := 0; i < sliceNum; i++ {
		detail.Reset()
		offset := int64(i) * SliceSize
		size := int64(SliceSize)
		if offset+size > fileSize {
			size = fileSize - offset
		}
		s := io.NewSectionReader(f, offset, size)
		io.CopyBuffer(writer, s, buf)
		hash := detail.Sum(nil)
		slices[i] = strings.ToUpper(hex.EncodeToString(hash))
		partNames[i] = base64.StdEncoding.EncodeToString(hash)
	}

	fileMD5 = hex.EncodeToString(global.Sum(nil))
	if sliceNum > 1 {
		h := md5.New()
		h.Write([]byte(strings.Join(slices, "\n")))
		sliceMD5 = hex.EncodeToString(h.Sum(nil))
	} else {
		sliceMD5 = fileMD5
	}
	return
}

func getDownloadURLByID(fileID string) (string, error) {
	req, err := apiGet("/getFileDownloadUrl.action", url.Values{"fileId": {fileID}})
	if err != nil {
		return "", err
	}

	var info struct {
		FileDownloadURL string `json:"fileDownloadUrl"`
	}
	if err := doAPI(req, &info); err != nil {
		return "", fmt.Errorf("获取下载链接失败: %w", err)
	}
	if info.FileDownloadURL == "" {
		return "", fmt.Errorf("获取下载链接为空")
	}
	return info.FileDownloadURL, nil
}

func getDownloadURL(path string) (string, error) {
	fileID, isDir, _, err := pathToID(path)
	if err != nil {
		return "", err
	}
	if isDir {
		return "", fmt.Errorf("路径指向的是目录而非文件: %s", path)
	}
	return getDownloadURLByID(fileID)
}

func cmdURL(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: yd url <文件路径>")
		os.Exit(1)
	}
	filePath := args[0]
	dlURL, err := getDownloadURL(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(formatDownloadCommand(dlURL, filepath.Base(filePath)))
}

func formatDownloadCommand(downloadURL, localName string) string {
	safeName := shellQuote(localName)
	safeURL := shellQuote(downloadURL)

	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("powershell -Command \"Invoke-WebRequest -Uri %s -OutFile %s\"", safeURL, safeName)
	default:
		return fmt.Sprintf("curl -L -o %s %s", safeName, safeURL)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
