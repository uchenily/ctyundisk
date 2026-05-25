package main

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	APIBase    = "https://api.cloud.189.cn"
	UploadBase = "https://upload.cloud.189.cn"

	ClientType = "TELEPC"
	Version    = "7.1.8.0"
	ChannelID  = "web_cloud.189.cn"

	RootFolder = "-11"
	SliceSize  = 10 * 1024 * 1024
)

type Session struct {
	Key          string `json:"sessionKey"`
	Secret       string `json:"sessionSecret"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type Config struct {
	Session *Session `json:"session,omitempty"`
}

var session *Session

func configPath() (string, error) {
	// exe, err := os.Executable()
	// if err != nil {
	// 	return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	// }
	// return filepath.Join(filepath.Dir(exe), "config.json"), nil

	// return "./config.json", nil

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("获取家目录失败:", err)
		return "", err
	}
	configPath := filepath.Join(home, ".yd.conf")
	return configPath, nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败 %s: %w\n请先执行 yd login <用户名> <密码>", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	if cfg.Session == nil || cfg.Session.Key == "" || cfg.Session.Secret == "" {
		return nil, fmt.Errorf("配置文件中缺少有效的 session.sessionKey / session.sessionSecret")
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
}

func hmacSha1(data, key string) string {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func aesEncryptECB(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	bs := block.BlockSize()
	padded := pkcs7Pad(data, bs)
	enc := make([]byte, len(padded))
	for i := 0; i < len(padded); i += bs {
		block.Encrypt(enc[i:i+bs], padded[i:i+bs])
	}
	return enc, nil
}

func pkcs7Pad(data []byte, bs int) []byte {
	pad := bs - len(data)%bs
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func randomUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func customEncode(v url.Values) string {
	if v == nil {
		return ""
	}
	var buf strings.Builder
	for k, vals := range v {
		for _, val := range vals {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(val)
		}
	}
	return buf.String()
}

func signRequest(req *http.Request) {
	now := time.Now()

	q := req.URL.Query()
	q.Set("rand", strconv.FormatInt(now.UnixMilli(), 10))
	q.Set("clientType", ClientType)
	q.Set("version", Version)
	q.Set("channelId", ChannelID)
	req.URL.RawQuery = q.Encode()

	date := now.Format(time.RFC1123)
	signData := fmt.Sprintf("SessionKey=%s&Operate=%s&RequestURI=%s&Date=%s",
		session.Key, req.Method, req.URL.Path, date)
	if req.URL.Host == "upload.cloud.189.cn" {
		signData += "&params=" + q.Get("params")
	}

	req.Header.Set("Date", date)
	req.Header.Set("SessionKey", session.Key)
	req.Header.Set("Signature", hmacSha1(signData, session.Secret))
	req.Header.Set("X-Request-ID", randomUUID())
	req.Header.Set("Accept", "application/json;charset=UTF-8")
	req.Header.Set("User-Agent", "desktop")
}

func apiGet(path string, params url.Values) (*http.Request, error) {
	req, err := http.NewRequest("GET", APIBase+path, nil)
	if err != nil {
		return nil, err
	}
	if len(params) > 0 {
		req.URL.RawQuery = params.Encode()
	}
	return req, nil
}

func doAPI(req *http.Request, result any) error {
	signRequest(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	if result != nil {
		return json.Unmarshal(body, result)
	}
	return nil
}

func buildUploadRequest(path string, params url.Values) (*http.Request, error) {
	plain := customEncode(params)
	enc, err := aesEncryptECB([]byte(plain), []byte(session.Secret[:16]))
	if err != nil {
		return nil, err
	}
	vals := make(url.Values)
	vals.Set("params", hex.EncodeToString(enc))

	req, err := http.NewRequest("GET", UploadBase+path+"?"+vals.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("decodefields", "familyId,parentFolderId,fileName,fileMd5,fileSize,sliceMd5,sliceSize,albumId,extend,lazyCheck,isLog")
	return req, nil
}

func doUploadAPI(req *http.Request, result any) error {
	client := &http.Client{Timeout: 0}
	signRequest(req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("上传API错误 HTTP %d: %s", resp.StatusCode, string(body))
	}
	if result != nil {
		return json.Unmarshal(body, result)
	}
	return nil
}

// ────────── Login ──────────

type qrRequest struct {
	Uuid       string `json:"uuid"`
	Encryuuid  string `json:"encryuuid"`
	Encodeuuid string `json:"encodeuuid"`
}

func cmdLogin(args []string) {
	fmt.Println("获取登录二维码...")

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	params := url.Values{
		// Use appid from cloud189
		"appId":      {"9317140619"},
		"clientType": {"10020"},
		"timeStamp":  {strconv.FormatInt(time.Now().UnixMilli(), 10)},
		"returnURL":  {"https://m.cloud.189.cn/zhuanti/2020/loginErrorPc/index.html"},
	}
	req, _ := http.NewRequest("GET", "https://cloud.189.cn/unifyLoginForPC.action?"+params.Encode(), nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "获取登录页面失败:", err)
		os.Exit(1)
	}
	resp.Body.Close()

	loc := resp.Request.Response
	if loc == nil || loc.Header.Get("location") == "" {
		fmt.Fprintln(os.Stderr, "未获取到登录重定向地址")
		os.Exit(1)
	}
	referer := loc.Header.Get("location")
	refURL, _ := url.Parse(referer)
	q := refURL.Query()
	appKey := q.Get("appId")
	lt := q.Get("lt")
	reqId := q.Get("reqId")

	ac, err := qrGetAppConf(client, referer, appKey, lt, reqId)
	if err != nil {
		fmt.Fprintln(os.Stderr, "获取应用配置失败:", err)
		os.Exit(1)
	}

	uuidReq, _ := http.NewRequest("GET", "https://open.e.189.cn/api/logbox/oauth2/getUUID.do?appId="+appKey, nil)
	uuidResp, err := client.Do(uuidReq)
	if err != nil {
		fmt.Fprintln(os.Stderr, "获取UUID失败:", err)
		os.Exit(1)
	}
	defer uuidResp.Body.Close()

	var qrReq qrRequest
	if err := json.NewDecoder(uuidResp.Body).Decode(&qrReq); err != nil {
		fmt.Fprintln(os.Stderr, "解析UUID失败:", err)
		os.Exit(1)
	}

	qrURL := fmt.Sprintf("https://open.e.189.cn/api/logbox/oauth2/image.do?REQID=%s&uuid=%s",
		reqId, qrReq.Encodeuuid)
	fmt.Println("\n请用浏览器打开以下链接，用手机天翼云盘扫码登录:")
	fmt.Println(qrURL)
	fmt.Println()

	fmt.Print("等待扫码...")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		state, err := qrCheckState(client, &qrReq, ac, referer, appKey, lt, reqId)
		if err != nil {
			fmt.Fprintln(os.Stderr, "\n查询状态失败:", err)
			os.Exit(1)
		}
		switch state.Status {
		case 0:
			session, err := getSessionDirect(client, state.RedirectUrl)
			if err != nil {
				fmt.Fprintln(os.Stderr, "获取会话失败:", err)
				os.Exit(1)
			}
			if err := saveConfig(&Config{Session: session}); err != nil {
				fmt.Fprintln(os.Stderr, "保存配置失败:", err)
				os.Exit(1)
			}
			fmt.Println("\n登录成功，配置已保存")
			return
		case -106:
			fmt.Print(".")
		case -11002:
			fmt.Println("\n已扫码，请在手机上确认...")
		default:
			fmt.Fprintf(os.Stderr, "\n未知状态: %d\n", state.Status)
			os.Exit(1)
		}
	}
}

type qrState struct {
	Status      int32  `json:"status"`
	RedirectUrl string `json:"redirectUrl"`
}

func qrCheckState(client *http.Client, qrReq *qrRequest, ac *appConfig, referer, appKey, lt, reqId string) (*qrState, error) {
	params := url.Values{
		"appId":      {appKey},
		"encryuuid":  {qrReq.Encryuuid},
		"uuid":       {qrReq.Uuid},
		"returnUrl":  {ac.ReturnURL},
		"clientType": {strconv.Itoa(ac.ClientType)},
		"timeStamp":  {strconv.FormatInt(time.Now().UnixMilli(), 10)},
		"cb_SaveName": {"0"},
		"isOauth2":   {strconv.FormatBool(ac.IsOauth2)},
		"state":      {""},
		"paramId":    {ac.ParamID},
		"date":       {time.Now().Format("2006-01-0215:04:059")},
	}
	u := "https://open.e.189.cn/api/logbox/oauth2/qrcodeLoginState.do?" + params.Encode()
	req, _ := http.NewRequest("POST", u, nil)
	req.Header.Set("Referer", referer)
	req.Header.Set("lt", lt)
	req.Header.Set("reqId", reqId)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var state qrState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func getSessionDirect(client *http.Client, toURL string) (*Session, error) {
	reqURL := APIBase + "/getSessionForPC.action"
	q := url.Values{
		"rand":        {strconv.FormatInt(time.Now().UnixMilli(), 10)},
		"clientType":  {ClientType},
		"version":     {Version},
		"channelId":   {ChannelID},
		"redirectURL": {toURL},
	}
	reqURL += "?" + q.Encode()

	req, _ := http.NewRequest("POST", reqURL, nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json;charset=UTF-8")
	req.Header.Set("User-Agent", "desktop")
	req.Header.Set("Referer", "https://api.cloud.189.cn")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求会话失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var s Session
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("解析会话响应失败 (redirectURL=%s): %w\n响应内容: %s", toURL, err, string(body[:200]))
	}
	if s.Key == "" || s.Secret == "" {
		return nil, fmt.Errorf("session 无效 (redirectURL=%s): %s", toURL, string(body[:200]))
	}
	return &s, nil
}

func qrGetAppConf(client *http.Client, referer, appKey, lt, reqId string) (*appConfig, error) {
	r, _ := http.NewRequest("POST", "https://open.e.189.cn/api/logbox/oauth2/appConf.do",
		strings.NewReader(url.Values{"version": {"2.0"}, "appKey": {appKey}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://open.e.189.cn")
	r.Header.Set("Referer", referer)
	r.Header.Set("Reqid", reqId)
	r.Header.Set("lt", lt)

	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data appConfig `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

type appConfig struct {
	AccountType string `json:"accountType"`
	AppKey      string `json:"appKey"`
	ClientType  int    `json:"clientType"`
	IsOauth2    bool   `json:"isOauth2"`
	MailSuffix  string `json:"mailSuffix"`
	ParamID     string `json:"paramId"`
	ReturnURL   string `json:"returnUrl"`
}

// ────────── Main ──────────

func main() {
	if len(os.Args) < 2 {
		fmt.Println(`yd - 天翼云盘命令行工具

用法:
  yd login                         登录并生成配置文件
  yd upload <文件路径> [-p 路径]     上传文件到云盘路径
  yd download <文件路径>             下载文件到当前目录
  yd url <文件路径>                  输出下载链接 (配合curl/wget使用)
  yd ls [路径]                      列出云盘目录内容

路径格式为 Unix 风格，如 /同步盘/yd , /我的文档
根目录用 / 表示`)
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "login" {
		cmdLogin(os.Args[2:])
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	session = cfg.Session

	switch cmd {
	case "upload":
		cmdUpload(os.Args[2:])
	case "download":
		cmdDownload(os.Args[2:])
	case "url":
		cmdURL(os.Args[2:])
	case "ls":
		cmdList(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", cmd)
		os.Exit(1)
	}
}

// ────────── Path Resolution ──────────

func pathToID(path string) (id string, isDir bool, size int64, err error) {
	if path == "" || path == "/" {
		return RootFolder, true, 0, nil
	}
	cleanPath := strings.TrimPrefix(path, "/")
	parts := strings.Split(cleanPath, "/")
	currentID := RootFolder
	for i, part := range parts {
		if part == "" {
			continue
		}
		isLast := (i == len(parts)-1)
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
		} else {
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
	}
	return currentID, true, 0, nil
}

// ────────── Upload ──────────

func cmdUpload(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: yd upload <本地文件路径> [-p 路径]")
		os.Exit(1)
	}

	localPath := args[0]
	parentPath := "/"

	remaining := args[1:]
	for i := 0; i < len(remaining); i++ {
		if remaining[i] == "-p" && i+1 < len(remaining) {
			parentPath = remaining[i+1]
			i++
		}
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

			putReq, err := http.NewRequest("PUT", partInfo.RequestURL, section)
			if err != nil {
				return err
			}
			for _, hdr := range strings.Split(partInfo.RequestHeader, "&") {
				idx := strings.Index(hdr, "=")
				if idx > 0 {
					putReq.Header.Set(hdr[:idx], hdr[idx+1:])
				}
			}

			fmt.Printf("\r上传分片 %d/%d...", i+1, sliceNum)
			putResp, err := http.DefaultClient.Do(putReq)
			if err != nil {
				return fmt.Errorf("上传分片 %d 失败: %w", i+1, err)
			}
			putResp.Body.Close()
			if putResp.StatusCode != 200 {
				return fmt.Errorf("上传分片 %d HTTP %d", i+1, putResp.StatusCode)
			}
		}
		fmt.Println()
	} else {
		fmt.Println("秒传成功！")
	}

	commitParams := make(url.Values)
	commitParams.Set("uploadFileId", uploadFileID)
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
	fmt.Printf("上传完成: %s\n%s\n", fileName, dlURL)
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
	path := args[0]
	dlURL, err := getDownloadURL(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(dlURL)
}

// ────────── Download ──────────

func cmdDownload(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "用法: yd download <文件路径>")
		os.Exit(1)
	}
	path := args[0]
	if err := doDownload(path); err != nil {
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

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	fmt.Printf("下载完成: %s (%d 字节)\n", localName, offset+written)
	return nil
}

// ────────── List ──────────

type cloudFile struct {
	ID   json.Number `json:"id"`
	Name string      `json:"name"`
	Size int64       `json:"size"`
	IsDir bool
}

type listResp struct {
	ResCode    int    `json:"res_code"`
	ResMessage string `json:"res_message"`
	FileListAO struct {
		Count        int         `json:"count"`
		FileListSize int         `json:"fileListSize"`
		FileList     []cloudFile `json:"fileList"`
		FolderList   []cloudFile `json:"folderList"`
	} `json:"fileListAO"`
}

func cmdList(args []string) {
	path := "/"
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

// use github.com/mattn/go-runewidth?
func displayWidth(s string) int {
    width := 0
    for _, r := range s {
        // 判断是否为宽字符（常见中文、全角字符）
        if (r >= 0x4E00 && r <= 0x9FFF) || // 基本汉字
            (r >= 0x3400 && r <= 0x4DBF) || // 扩展 A
            (r >= 0x20000 && r <= 0x2A6DF) || // 扩展 B
            (r >= 0xF900 && r <= 0xFAFF) || // 兼容汉字
            (r >= 0xFF00 && r <= 0xFFEF) { // 全角标点/数字/字母
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

    // 构建一个结构体保存每条记录
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

    // 计算最大显示宽度
    maxNameWidth := 0
    for _, it := range items {
        w := displayWidth(it.Name)
        if w > maxNameWidth {
            maxNameWidth = w
        }
    }

    // 对齐打印
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
