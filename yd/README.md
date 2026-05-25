# yd

天翼云盘命令行上传下载工具，纯 Go 标准库实现，无第三方依赖。

## 安装

```bash
git clone https://github.com/example/yd.git
cd yd
go build -o yd .
```

## 用法

```bash
# 登录（自动生成配置文件）
yd login [用户名] [密码]

# 列出文件
yd ls                  # 根目录
yd ls <文件夹ID>        # 指定文件夹

# 上传（完成后输出下载链接）
yd upload <文件路径>
yd upload <文件路径> -p <文件夹ID>

# 下载
yd download <文件名>

# 仅输出下载链接
yd url <文件名>
```

### 示例

```bash
# 首次使用先登录
yd login 13800138000

# 上传并获取下载链接
yd upload video.mp4

# 上传到「我的文档」
yd upload doc.pdf -p -15

# 用 curl 下载
yd url video.mp4 | xargs curl -o video.mp4 -L

# 断点续传下载
yd download video.mp4
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

- 内置登录，无需第三方工具生成配置
- 配置文件自动保存至可执行文件同目录
- 大文件自动分片上传（10MB/片）
- 秒传检测，已存在的文件免上传
- 断点续传下载
- 纯 Go 标准库，零依赖
