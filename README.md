# yd

天翼云盘命令行上传下载工具.

## 安装

```bash
git clone https://gitee.com/uchenily/ctyundisk.git
cd ctyundisk
make
```

## 配置文件

默认使用 `~/.yd.conf`，也可通过 `YD_CONFIG_PATH` 指定其它路径。

## 用法

```bash
# 登录（自动生成配置文件）
yd login [用户名] [密码]

# 默认列出配置中的 workDir；未配置时列根目录
yd ls

# 列出文件
yd ls /                # 根目录
yd ls <文件路径>       # 支持绝对路径或相对 workDir 的路径

# 上传（完成后输出下载链接）
yd upload <文件路径>
yd upload <文件路径> <文件夹>

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
yd upload doc.pdf /我的文档

# 配置 ~/.yd.conf 后：
# { "workDir": "/同步盘", "session": { ... } }
# 下面几条都会基于 /同步盘
yd ls
yd upload doc.pdf
yd download 资料/video.mp4

# 直接下载
yd url video.mp4

# 断点续传下载
yd download video.mp4
```

## 特性

- 二维码扫码即可登录生成配置
- 新生成的配置默认写入 `workDir=/同步盘`
- 大文件自动分片上传（10MB/片）
- 秒传检测，已存在的文件免上传
- 断点续传下载

## 参考项目

https://github.com/gowsp/cloud189
