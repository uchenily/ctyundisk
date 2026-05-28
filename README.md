# yd

天翼云盘命令行上传下载工具.

## 安装

```bash
git clone https://gitee.com/uchenily/ctyundisk.git
cd ctyundisk
make
```

## 用法

```bash
# 登录（自动生成配置文件）
yd login [用户名] [密码]

# 列出文件
yd ls                  # 根目录
yd ls <文件路径>       # 指定文件路径

# 上传（完成后输出下载链接）
yd upload <文件路径>
yd upload <文件路径> -p <文件夹>

# 下载
yd download <文件路径>

# 仅输出下载链接
yd url <文件路径>
```

### 示例

```bash
# 首次使用先登录
yd login

# 上传并获取下载链接
yd upload video.mp4

# 上传到「我的文档」
yd upload doc.pdf -p /我的文档

# 用 curl 下载
yd url video.mp4 | xargs curl -o video.mp4 -L

# 断点续传下载
yd download video.mp4
```

## 特性

- 二维码扫码即可登录生成配置
- 大文件自动分片上传（10MB/片）
- 秒传检测，已存在的文件免上传
- 断点续传下载

## 参考项目

https://github.com/gowsp/cloud189
