# SuperLocal

局域网聊天和传文件。同一网段的人用浏览器打开终端里打印的地址即可进入。

## 构建前端

前端构建需要 Node 24。已安装的 Vite 8 不能在 Node 20.10 上运行。

```bash
export PATH="/usr/local/opt/node@24/bin:$PATH"
cd web && npm install && npm run build
```

## 编译并启动

在仓库根目录执行：

```bash
CGO_ENABLED=0 go build -o superlocal ./cmd/superlocal
./superlocal
```

终端会打印聊天页地址、管理页地址、管理员账号 `admin`，以及当前密码。把聊天地址发给要聊天的人。管理地址只给管理员。

## 账号

Key 就是账号。聊天没有单独的密码。拿到某个 Key 的人，就是这个用户。用户能看到自己的 Key；其他人只显示昵称。

## 保留时间

消息和文件保留 48 小时。清理在启动时执行一次，之后每 48 小时再执行一次。

## 服务管理壳

`desktop/` 是 Electron 管理程序。对方电脑安装后不需要再安装 Go 或 Node。双击只打开管理窗口，点「启动」才运行服务。默认端口是 `8080`，可以在窗口里修改。

打安装包的电脑需要 Go 和 Node 24。macOS 安装包在 Mac 上打，Windows 安装包在 Windows 上打。

macOS：

```bash
export PATH="/usr/local/opt/node@24/bin:$PATH"
CGO_ENABLED=0 go build -o desktop/binaries/superlocal ./cmd/superlocal
cd desktop && npm install && npm test && npm run pack
```

Windows 在 `cmd` 中：

```bat
set CGO_ENABLED=0
go build -o desktop\binaries\superlocal.exe .\cmd\superlocal
cd desktop && npm install && npm test && npm run pack
```

开发时可以不打包，先编译服务程序，再在 `desktop/` 执行 `npm start`。macOS 安装包没有签名，第一次打开时按系统提示允许该应用。
