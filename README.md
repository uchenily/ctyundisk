# yd

天翼云盘命令行工具 —— 扫码登录、上传、下载、列目录、删除，一条命令搞定。

## 安装

需要 Go 1.22+。

```bash
git clone https://gitee.com/uchenily/ctyundisk.git
cd ctyundisk
make
```

编译产物输出到 `build/yd`。

## 快速开始

```bash
# 1. 扫码登录（终端会显示二维码，用手机天翼云盘/微信/支付宝扫码）
yd login

# 2. 列出云盘文件
yd ls

# 3. 上传文件
yd upload video.mp4

# 4. 下载文件
yd download 资料/video.mp4
```

## 命令

```
yd login                       扫码登录，生成配置文件
yd upload <本地文件> [云盘目录] 上传文件，完成后输出下载链接
yd download <云盘文件路径>     下载文件到当前目录（支持断点续传）
yd url <云盘文件路径>          输出直接下载命令（curl / PowerShell）
yd ls [云盘路径]               列出目录内容
yd remove <云盘路径...>        删除文件或目录（支持批量）
```

路径格式为 Unix 风格，根目录用 `/` 表示，如 `/同步盘/文档`。

## 配置文件

默认路径 `~/.yd.conf`，可通过环境变量 `YD_CONFIG_PATH` 指定其他位置。

配置示例：

```json
{
  "workDir": "/同步盘",
  "session": {
    "sessionKey": "...",
    "sessionSecret": "...",
    "accessToken": "...",
    "refreshToken": "..."
  }
}
```

- `workDir`：默认工作目录。设置后，所有相对路径会基于该目录解析；未设置时默认为根目录 `/`。
- `session`：由 `yd login` 自动写入，无需手动编辑。

## 示例

```bash
# 上传到指定目录
yd upload doc.pdf /我的文档

# 配置了 workDir=/同步盘 后，相对路径自动基于该目录
yd ls                    # 列出 /同步盘
yd upload doc.pdf        # 上传到 /同步盘
yd download 资料/video.mp4  # 下载 /同步盘/资料/video.mp4

# 获取下载命令（不实际下载）
yd url video.mp4

# 断点续传：中断后再次执行同一命令即可续传
yd download video.mp4

# 批量删除
yd remove 旧文件.txt /同步盘/旧目录
```

## 特性

- 扫码登录 —— 终端直接渲染二维码，无需浏览器
- 大文件分片上传（10 MB/片），带进度条
- 秒传检测，云端已有文件免上传
- 断点续传下载
- 批量删除文件或目录
- 跨平台下载命令输出（Linux/macOS 输出 curl，Windows 输出 PowerShell）

## 参考项目

- https://github.com/gowsp/cloud189

## 社区&交流

欢迎到 [LINUX DO](https://linux.do/) 社区分享交流。
