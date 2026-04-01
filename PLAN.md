# JSON Compare — 项目计划（Sprint 划分）

说明：本计划用于把当前本地项目逐步扩展为类似 jsoncompare.com 的语义化 JSON 对比工具。每个 Sprint 都包含目标、产出、验收条件和风险/备注。

## 总目标
- 在本地运行（隐私优先），提供语义化 JSON 比较（忽略格式、键顺序），并可视化显示 additions / deletions / modifications。
- 支持粘贴与文件上传、差异逐项展示、跳转定位、行内高亮、过滤规则、大文件支持与最终离线打包选项。

---

## Sprint 1 — 基础（已完成）
目标：实现最小可用产品（MVP）。
产出：
- 后端（Go）提供 `/compare` 接口，返回 prettyA/prettyB、asciiDiff、patch。
- 前端（纯 HTML/JS）并排显示两个可编辑区域，支持文件上传、粘贴、下载比较结果。
验收条件：在本地运行 `go run main.go` 后能打开页面并比较两个 JSON。
风险：无外部依赖（除了 gojsondiff），功能基线稳定。

## Sprint 2 — 解析错误友好化与客户端校验（已实现）
目标：提高可用性，避免对无效 JSON 误比较。
产出：
- 后端检测 JSON 解析错误并返回 `errorA` / `errorB`（包含 message、line、column）。
- 前端在粘贴/编辑时做即时校验并显示红色错误提示，支持跳转到对应行，若有解析错误则阻止比较。
验收条件：粘贴无效 JSON 时前端显示错误，点击跳转能定位到错误行；服务器端也会返回解析错误并显示。

## Sprint 3 — 行内高亮与同步滚动（短期优先）
目标：在两侧编辑区对比时高亮差异行，支持同步滚动与并列视觉对比。
产出：
- 在 Pretty A/B 中对差异行做背景色标注（added/removed/modified）。
- 双向或单向同步滚动（可在设置中切换）。
选项：
- 轻量方案：在现有 textarea 上方使用 overlay 或把 textarea 换成 contenteditable/div 渲染高亮（实现复杂度中等）。
- 进阶方案：引入 CodeMirror/Monaco 编辑器做精细高亮与性能优化（需要引入库）。
验收条件：差异项在两个面板中可视化高亮，滚动可联动。
风险：引入编辑器库会增加前端复杂度与包体积。

## Sprint 4 — 高级比较规则与过滤（中期）
目标：支持常见比较选项，满足多场景需求。
产出：
- 忽略指定字段（例如 timestamp、id）
- 忽略数组顺序或把数组按键索引进行匹配
- 数值容差（浮点近似比较）
- 过滤视图（只看 added/removed/modified）
验收条件：UI 可以配置规则并影响后端比较结果。
风险：比较规则复杂度较高，需设计清晰的 API 与用户交互。

## Sprint 5 — 大文件与性能（中期/后期）
目标：提升对大 JSON（如 50MB API dump）的处理能力。
产出：
- 增加上传/处理限制，支持进度条
- 后端可使用临时文件或流式解析，避免一次性 OOM
- 可取消长时间运行的比较任务
验收条件：成功比较大于 20MB 的 JSON（取决机器内存），并能显示进度/取消。
风险：实现复杂，需要更多测试与可能的原生扩展。

## Sprint 6 — 本地打包与离线（可选）
目标：提供原生 macOS 应用或单文件分发，满足对敏感数据的完全离线需求。
产出：
- 使用 Electron / Tauri 打包的桌面应用，或提供 CLI 工具
验收条件：应用可在目标平台离线启动并完成比较任务。
风险：打包配置与跨平台支持工作量大。

---

## 优先级与建议路线
优先级（推荐）：Sprint 2（已完成） → Sprint 3 → Sprint 4 → Sprint 5 → Sprint 6。
当前建议：先实现 Sprint 3（行内高亮与同步滚动），可显著提升可读性；可选同时评估是否引入 CodeMirror/Monaco。

## 开发/运行说明（快速开始）
在项目根目录下：

```bash
cd /Users/outsider/Downloads/Golang_Development/json_compare
# 获取依赖
go get github.com/yudai/gojsondiff
go mod tidy
# 运行
go run main.go
# 打开浏览器访问
open http://localhost:8080
```

## 下一步（我将执行）
- 如果你确认 Sprint 3，我将：
  1. 评估并选择高亮实现方式（轻量 overlay vs 编辑器库），并写出实现方案（短文）。
  2. 根据选项开始实现：先尝试轻量实现并评估效果；如不足，再切换到 CodeMirror。
  3. 实现后写入前端并测试同步滚动与高亮。

---

如果需要我现在把 Sprint 3 的实现方案写成更详细的设计文档，回复“开始 Sprint 3 设计”。
