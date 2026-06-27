package main

import "encoding/json"

const (
	APIBase    = "https://api.cloud.189.cn"
	UploadBase = "https://upload.cloud.189.cn"

	ClientType = "TELEPC"
	Version    = "7.1.8.0"
	ChannelID  = "web_cloud.189.cn"

	RootFolder = "-11"
	SliceSize   = 10 * 1024 * 1024

	defaultWorkDir = "/同步盘"
	qrAutoSaveName = "3"
)

type Session struct {
	LoginName    string `json:"loginName,omitempty"`
	Key          string `json:"sessionKey"`
	Secret       string `json:"sessionSecret"`
	KeepAlive    int    `json:"keepAlive,omitempty"`
	FileDiffSpan int    `json:"getFileDiffSpan,omitempty"`
	UserInfoSpan int    `json:"getUserInfoSpan,omitempty"`
	FamilyKey    string `json:"familySessionKey,omitempty"`
	FamilySecret string `json:"familySessionSecret,omitempty"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type Config struct {
	Session *Session `json:"session,omitempty"`
	WorkDir string    `json:"workDir,omitempty"`
}

type qrRequest struct {
	Uuid       string `json:"uuid"`
	Encryuuid  string `json:"encryuuid"`
	Encodeuuid string `json:"encodeuuid"`
}

type qrState struct {
	Status      int32  `json:"status"`
	RedirectUrl string `json:"redirectUrl"`
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

type cloudFile struct {
	ID    json.Number `json:"id"`
	Name  string      `json:"name"`
	Size  int64       `json:"size"`
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

var session *Session
var config *Config
