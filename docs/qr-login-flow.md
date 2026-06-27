# 扫码登录流程

本文基于 `test.js` 和当前 CLI 实现整理。

## 总体流程

1. 访问 `https://cloud.189.cn/unifyLoginForPC.action`
2. 跟随重定向，拿到 `appId`、`lt`、`reqId`
3. 调用 `appConf.do` 获取扫码配置
4. 调用 `getUUID.do` 获取二维码所需的 `uuid / encryuuid / encodeuuid`
5. 访问 `image.do` 生成二维码图片
6. 轮询 `qrcodeLoginState.do`
7. 成功后用 `redirectUrl` 换取会话

## 1. 入口页

`GET https://cloud.189.cn/unifyLoginForPC.action`

常见查询参数：

- `appId=9317140619`
- `clientType=10020`
- `timeStamp=<毫秒时间戳>`
- `returnURL=https://m.cloud.189.cn/zhuanti/2020/loginErrorPc/index.html`

作用：

- 初始化登录入口
- 触发跳转到 `open.e.189.cn` 的统一登录页
- 响应头里会带后续要用的 `lt` 和 `reqId`

## 2. 应用配置

`POST https://open.e.189.cn/api/logbox/oauth2/appConf.do`

请求体：

```text
version=2.0&appKey=<appKey>
```

常见请求头：

- `Origin: https://open.e.189.cn`
- `Referer: <上一跳重定向地址>`
- `Reqid: <reqId>`
- `lt: <lt>`

返回 `data` 中常见字段：

- `returnUrl`
- `clientType`
- `isOauth2`
- `state`
- `paramId`
- `accountType`
- `mailSuffix`
- `appKey`

这些值会原样进入后面的扫码轮询请求。

## 3. 获取二维码参数

`GET https://open.e.189.cn/api/logbox/oauth2/getUUID.do?appId=<appKey>`

返回 JSON 里关键字段：

- `uuid`
- `encryuuid`
- `encodeuuid`

其中：

- `uuid` 是轮询状态所需标识
- `encryuuid` 参与状态查询
- `encodeuuid` 用于 `image.do`

## 4. 二维码图片

`GET https://open.e.189.cn/api/logbox/oauth2/image.do?REQID=<reqId>&uuid=<encodeuuid>`

这是实际展示给用户扫描的二维码图片地址。

## 5. 状态轮询

`POST https://open.e.189.cn/api/logbox/oauth2/qrcodeLoginState.do`

请求参数：

- `appId=<appKey>`
- `encryuuid=<encryuuid>`
- `uuid=<uuid>`
- `returnUrl=<appConf.returnUrl>`
- `clientType=<appConf.clientType>`
- `timeStamp=<毫秒时间戳>`
- `cb_SaveName=<自动登录保存档位>`
- `isOauth2=<appConf.isOauth2>`
- `state=<appConf.state>`
- `paramId=<appConf.paramId>`
- `date=<浏览器风格时间戳>`

请求头：

- `Referer: <统一登录页地址>`
- `lt: <lt>`
- `reqId: <reqId>`

## 6. `cb_SaveName` 的含义

在网页里：

- `data-index=0` 表示未勾选自动登录
- `data-index=1` 表示勾选自动登录
- 如果 `data-index=1`，前端会把下拉框选中的 `data-time` 作为 `cb_SaveName` 发送

当前 CLI 直接固定发送：

```text
cb_SaveName=3
```

也就是你要求的 30 天档位。

## 7. 轮询返回值

从前端代码看，常见状态含义：

- `status=-106`：继续等待
- `status=-11002`：已扫码，等待手机确认
- `status=0`：登录成功，返回 `redirectUrl`
- `status=-134`：需要切到密码登录
- `result=-20099`：页面或二维码过期

注意：前端实际是同时看 `status` 和 `result`，不是只有一个字段。

## 8. 会话交换

`POST https://api.cloud.189.cn/getSessionForPC.action`

查询参数：

- `rand=<毫秒时间戳>`
- `clientType=TELEPC`
- `version=7.1.8.0`
- `channelId=web_cloud.189.cn`
- `redirectURL=<qrcodeLoginState 返回的 redirectUrl>`

请求头：

- `Content-Type: application/x-www-form-urlencoded`
- `Accept: application/json;charset=UTF-8`
- `User-Agent: desktop`
- `Referer: https://api.cloud.189.cn`

返回里会带：

- `sessionKey`
- `sessionSecret`
- `accessToken`
- `refreshToken`

CLI 会把这份 session 保存到 `~/.yd.conf`。

新生成的配置会默认写入：

- `workDir=/同步盘`

## 9. CLI 当前实现要点

- `login` 命令只走扫码登录
- 轮询间隔是 3 秒
- 登录成功后保存 `session`
- `cb_SaveName` 已固定为 `3`

## 10. 代码位置

- `login.go`
- `http.go`
- `path.go`
- `upload.go`
- `download.go`
- `list.go`
