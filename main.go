package main

import (
	"fmt"
	"os"
)

func usage() {
	fmt.Println(`yd - 天翼云盘命令行工具

用法:
  yd login                           扫码登录并生成配置文件
  yd upload <文件路径> [路径]        上传文件到云盘路径
  yd download <文件路径>             下载文件到当前目录
  yd url <文件路径>                  输出直接下载命令
  yd ls [路径]                       列出云盘目录内容

路径格式为 Unix 风格，如 /同步盘/yd , /我的文档
配置文件可选 workDir 作为默认目录，此时相对路径会基于该目录解析
根目录用 / 表示
可通过环境变量 YD_CONFIG_PATH 指定配置文件路径`)
}

func isHelpArg(s string) bool {
	return s == "-h" || s == "--help" || s == "help"
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if isHelpArg(cmd) {
		usage()
		return
	}
	if len(os.Args) > 2 && isHelpArg(os.Args[2]) {
		usage()
		return
	}
	if cmd == "login" {
		cmdLogin(os.Args[2:])
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	config = cfg
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
