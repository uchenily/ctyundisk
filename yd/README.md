# yd

天翼云盘命令行上传下载工具，纯 Go 标准库实现，无第三方依赖。

## 安装

```bash
git clone https://github.com/example/yd.git
cd yd
go build -o yd .
```

## 配置

将 `config.json` 放在 `yd` 可执行文件同一目录下：

```json
{
  "session": {
    "sessionKey": "xxx",
    "sessionSecret": "xxx",
    "accessToken": "xxx",
    "refreshToken": "xxx"
  }
}
```

配置文件可通过 [cloud189](https://github.com/gowsp/cloud189) 等工具登录生成，位于 `~/.config/cloud189/config.json`。

## 用法

```bash
yd ls                  # 列出根目录文件
yd ls <文件夹ID>        # 列出指定文件夹

yd upload <文件路径>    # 上传到根目录，完成后输出下载链接
yd upload <文件路径> -p <文件夹ID>  # 上传到指定文件夹

yd download <文件名>    # 下载文件到当前目录（支持断点续传）

yd url <文件名>         # 仅输出下载链接，配合 curl/wget 使用
```

### 示例

```bash
# 上传并获取下载链接
yd upload video.mp4

# 上传到「我的文档」
yd upload doc.pdf -p -15

# 下载
yd download video.mp4

# 用 curl 下载
yd url video.mp4 | xargs curl -o video.mp4 -L

# 用 wget 下载
yd url video.mp4 | xargs wget -O video.mp4
```

### 常用文件夹 ID

| 名称     | ID   |
| -------- | ---- |
| 全部文件 | -11  |
| 我的图片 | -12  |
| 我的视频 | -13  |
| 我的音乐 | -14  |
| 我的文档 | -15  |
| 我的应用 | -16  |

## 特性

- 大文件自动分片上传（10MB/片）
- 秒传检测，已存在的文件免上传
- 断点续传下载
- 纯 Go 标准库，零依赖
