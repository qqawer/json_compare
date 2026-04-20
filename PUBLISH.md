# 发布与使用说明（qqawer/json_compare）

## 前置条件

- 安装 Docker（Docker Desktop 推荐）。
- 已登录 Docker Hub：`docker login`（使用账号 `qqawer`）。
- 若要构建 multi‑arch，需要 Docker Buildx（通常随 Docker Desktop 可用）。

## 关键文件

- `Dockerfile` — 镜像构建配置（会把 `./frontend` 复制进镜像以供静态文件服务）。
- `build_and_push_multiarch.sh` — 使用 `docker buildx` 构建并推送 multi‑arch 镜像（linux/amd64, linux/arm64）。
- `push_to_dockerhub.sh` — 简单的单架构 build & push 脚本。
- `main.go` — 后端服务代码（包含静态文件服务与 `/compare` 接口）。
- `frontend/index.html` — 前端静态页面（含下载按钮）。

## 单架构：构建并推送（快速）

在项目根目录运行：

- 使用脚本：
  ```bash
  ./push_to_dockerhub.sh qqawer json_compare latest
  ```

- 或手动：
  ```bash
  docker build -t qqawer/json_compare:latest .
  docker push qqawer/json_compare:latest
  ```

适用场景：仅目标主机与当前构建平台相同时（例如都为 x86_64）。

## 多架构（推荐）

在项目根目录运行：

```bash
./build_and_push_multiarch.sh qqawer json_compare latest
```

等价命令（手动）：

```bash
docker buildx create --name multi-builder --use    # 如尚未创建
docker buildx build --platform linux/amd64,linux/arm64 -t qqawer/json_compare:latest --push .
```

说明：构建并推送后，Docker Hub 会保存一个 multi‑arch manifest，目标机器会自动拉取适配其平台的镜像。

## 本地运行 / 测试

- 拉取镜像：
  ```bash
  docker pull qqawer/json_compare:latest
  ```
- 运行并暴露端口 8090：
  ```bash
  docker run --rm -p 8090:8090 qqawer/json_compare:latest
  ```
- 在浏览器打开： `http://localhost:8090`

更改宿主端口：

```bash
docker run --rm -p 8080:8090 qqawer/json_compare:latest
```

改变容器内监听端口（需同时设置 `PORT` 环境变量）：

```bash
docker run --rm -p 8081:8081 -e PORT=8081 qqawer/json_compare:latest
```

查看后台容器日志：

```bash
docker logs -f <container-name-or-id>
```

停止并移除容器：

```bash
docker stop <container-id>
docker rm <container-id>
```

## 在 Windows 上运行（PowerShell / CMD）

- 安装 Docker Desktop 并登录：`docker login`。
- 拉取并运行：
  ```powershell
  docker pull qqawer/json_compare:latest
  docker run --rm -p 8090:8090 qqawer/json_compare:latest
  ```
- 浏览器访问： `http://localhost:8090`

## 常见故障排查

- 访问根路径 404：确认镜像内 `/frontend` 存在并且容器已正常启动（查看容器日志）。
- 构建错误：检查 `go.mod`、网络与 `go mod download` 是否成功。
- 推送失败（权限）：确保 `docker login` 使用的账号为 `qqawer` 并有该仓库的推送权限。

## 可选：清理依赖

如果 `go.mod` 包含未使用的模块：

```bash
go mod tidy
```

然后测试编译：

```bash
go build
```

## 建议的 Git 提交信息（对应文件）

- `Dockerfile`  
  提交信息："Dockerfile: include frontend static files and support TARGETARCH for multi-arch build"

- `build_and_push_multiarch.sh`  
  提交信息："ci: add buildx script to build & push linux/amd64 and linux/arm64 multi-arch images"

- `push_to_dockerhub.sh`  
  提交信息："scripts: add push_to_dockerhub.sh for single-arch build & push"

- `main.go`  
  提交信息："server: improve error handling, produce RFC-6902 and annotated patches, respect PORT env var"

- `frontend/index.html`  
  提交信息："frontend: add patch download buttons; avoid auto-format on input; wire downloads to server response"

- `go.mod`（如有变动）  
  提交信息："deps: tidy or update dependencies" 或 "deps: remove unused dependency"

示例一次性提交：

```bash
git add Dockerfile build_and_push_multiarch.sh push_to_dockerhub.sh main.go frontend/index.html go.mod
git commit -m "chore: docker + ci scripts; server/frontend updates for patch downloads" 
```

## 新增选项：忽略空白差异（ignoreWhitespace）

为了减少因换行或多空格导致的噪声差异，前端增加了一个“忽略空白差异”开关（默认开启）。该选项会在比较字符串时折叠连续空白（包括换行、制表和多个空格）为单个空格再进行比较，从而把仅因格式化不同而造成的差异视为相同。

- 前端 UI：在主页面 `frontend/index.html` 顶部控制区可见复选框 `忽略空白差异（默认开启）`，用户可以随时切换。
- 后端 API：在 POST `/compare` 时，表单字段 `ignoreWhitespace` 会被发送（默认值 `true`）。示例 FormData（前端自动发送）：

```text
fileA: <file or text>
fileB: <file or text>
ignoreWhitespace: true
```

- 注意事项：折叠空白会丢失某些多行文本的语义（例如段落分隔、日志或诗歌），因此该选项是可配置的。如果你希望保持换行敏感，请在页面取消勾选该复选框。
