package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func cmdLogin(args []string) {
	fmt.Println("获取登录二维码...")

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	workDir := defaultWorkDir
	if cfg, err := loadStoredConfig(); err == nil && cfg.WorkDir != "" {
		workDir = cleanCloudPath(cfg.WorkDir)
	}

	params := url.Values{
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
	fmt.Println("\n请用手机天翼云盘/微信/支付宝扫码登录:")
	if err := printQRCode(client, qrURL); err != nil {
		fmt.Fprintln(os.Stderr, "终端二维码渲染失败:", err)
		fmt.Println("可回退为直接打开该链接查看二维码:")
		fmt.Println(qrURL)
	}
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
			if err := saveConfig(&Config{Session: session, WorkDir: workDir}); err != nil {
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

func qrCheckState(client *http.Client, qrReq *qrRequest, ac *appConfig, referer, appKey, lt, reqId string) (*qrState, error) {
	params := url.Values{
		"appId":       {appKey},
		"encryuuid":   {qrReq.Encryuuid},
		"uuid":        {qrReq.Uuid},
		"returnUrl":   {ac.ReturnURL},
		"clientType":  {strconv.Itoa(ac.ClientType)},
		"timeStamp":   {strconv.FormatInt(time.Now().UnixMilli(), 10)},
		"cb_SaveName": {qrAutoSaveName},
		"isOauth2":    {strconv.FormatBool(ac.IsOauth2)},
		"state":       {""},
		"paramId":     {ac.ParamID},
		"date":        {time.Now().Format("2006-01-0215:04:059")},
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

func printQRCode(client *http.Client, qrURL string) error {
	req, err := http.NewRequest("GET", qrURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return err
	}

	bounds := img.Bounds()
	qrWidth := bounds.Dx()
	termRows, termCols := terminalSize()
	maxModules := max(1, (termRows*3)/4)
	if termCols > 0 {
		maxModules = min(maxModules, termCols/2)
	}
	scale := 1
	if qrWidth > maxModules {
		scale = (qrWidth + maxModules - 1) / maxModules
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 * scale {
		var line strings.Builder
		for x := bounds.Min.X; x < bounds.Max.X; x += scale {
			topDark := isDark(img.At(x, y))
			bottomDark := false
			bottomY := y + scale
			if bottomY < bounds.Max.Y {
				bottomDark = isDark(img.At(x, bottomY))
			}

			switch {
			case topDark && bottomDark:
				line.WriteString("██")
			case topDark:
				line.WriteString("▀▀")
			case bottomDark:
				line.WriteString("▄▄")
			default:
				line.WriteString("  ")
			}
		}
		fmt.Println(line.String())
	}
	return nil
}

func terminalSize() (rows, cols int) {
	rows, cols = terminalSizeFromEnv()
	if rows > 0 && cols > 0 {
		return rows, cols
	}

	ws, err := getWinSize()
	if err == nil {
		if rows <= 0 && ws.Row > 0 {
			rows = int(ws.Row)
		}
		if cols <= 0 && ws.Col > 0 {
			cols = int(ws.Col)
		}
	}
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	return rows, cols
}

func terminalSizeFromEnv() (rows, cols int) {
	rows, _ = strconv.Atoi(strings.TrimSpace(os.Getenv("LINES")))
	cols, _ = strconv.Atoi(strings.TrimSpace(os.Getenv("COLUMNS")))
	return rows, cols
}

func getWinSize() (*winsize, error) {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno != 0 {
		return nil, errno
	}
	return ws, nil
}

func isDark(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r+g+b < 3*0x8000
}
