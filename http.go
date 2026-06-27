package main

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

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
