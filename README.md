# ntpl

> 模板持续同步引擎 — 让模板持续演进，项目安全跟随

**ntpl**（nil template tool）是一个跨平台 CLI 工具，用于将 Git 模板仓库的更新安全同步到已有项目中。

适用于任意 Git 仓库：后端、前端、微服务脚手架、基础设施模板等。

## 为什么需要 ntpl？

| | 传统模板工具 | ntpl |
|---|---|---|
| 生成项目 | ✅ | ✅ |
| 后续同步模板更新 | ❌ | ✅ |
| 选择性同步 | ❌ | ✅ |
| 多模板源 | ❌ | ✅ |

## 功能

| 功能 | 说明 |
|------|------|
| 多模板源 | 一个项目可从多个 Git 仓库同步 |
| ref 支持 | branch / tag / commit hash |
| include / exclude | 按目录和文件精确控制同步范围 |
| .ntplignore | 类 .gitignore 语法的排除规则 |
| diff | 查看模板与项目的文件差异 |
| dry-run | 预览变更，不修改任何文件 |
| 交互式同步 | 逐文件确认是否覆盖 |
| 版本锁定 | .ntpl.lock 记录同步的 commit hash |
| status | 查看各模板同步状态及远程更新 |
| 跨平台 | 纯 Go 实现，无 rsync/diff 等外部依赖 |
| 模板变量替换 | `{ntpl:name}` 占位符，sync 时自动替换 |
| hook 支持 | sync 前后执行自定义脚本 |
| 远程配置源 | 从模板仓库读取默认 sync/vars 配置 |
| pack | 打包项目为模板，自动替换值为 `{ntpl:key}` 占位符 |
| replace | 替换源仓库的值为自己的值（适用于无占位符的仓库） |
| 声明式检测规则 | YAML 规则自动识别项目变量，支持自定义扩展 |
| replace 排除 | replace 自动跳过 vendor 等第三方目录，可配置 |

**安全保证：** 只同步 include 范围内的文件；不删除项目中有但模板中没有的文件；exclude 和 .ntplignore 中的文件不会被触碰。

## 安装

```bash
go install github.com/nilorg/ntpl@latest
```

或从源码构建：

```bash
git clone https://github.com/nilorg/ntpl
cd ntpl
make build       # 产物在 bin/ntpl
```

## 快速开始

```bash
# 1. 初始化（生成 .ntpl.yaml）
ntpl init --repo https://github.com/your-org/template.git

# 2. 编辑 .ntpl.yaml 配置 include/exclude（可选）

# 3. 预览变更
ntpl sync --dry-run

# 4. 同步
ntpl sync

# 5. 查看状态
ntpl status
```

## 命令

| 命令 | 说明 |
|------|------|
| `ntpl init --repo <url>` | 初始化配置文件 |
| `ntpl init --repo <url> --force` | 强制覆盖已有配置 |
| `ntpl sync` | 同步模板到项目 |
| `ntpl sync --dry-run` | 预览模式，不修改文件 |
| `ntpl sync -i` | 交互式，逐文件确认 |
| `ntpl diff` | 查看模板与项目差异 |
| `ntpl status` | 查看同步状态及远程更新 |
| `ntpl pack -o <dir> --suggest` | 打包项目为模板 |
| `ntpl pack -o <dir> --var k=v` | 指定变量打包（多个变量重复 `--var`） |
| `ntpl replace` | 按配置替换源值 |
| `ntpl replace --suggest` | 交互式检测并替换 |

## 配置

### .ntpl.yaml

```yaml
templates:
  - name: default
    repo: https://github.com/your-org/template.git
    ref: main

sync:
  include:          # 为空则同步整个模板
    - src
    - configs
  exclude:
    - .env
    - config.local.yaml
  vars:               # 模板变量，替换 {ntpl:key}
    project_name: my-app
    org: nilorg
    port: "8080"
  hooks:              # sync 前后执行脚本
    before: ./scripts/backup.sh
    after: ./scripts/gen.sh

replace:
  exclude:            # replace 时跳过的目录（默认 vendor, node_modules）
    - vendor
    - node_modules
  rules:              # 替换规则
    org: nilorg
    project_name:
      from: "template-project"
      to: "my-app"
```

多模板源：

```yaml
templates:
  - name: backend
    repo: https://github.com/your-org/backend-template.git
    ref: main
  - name: infra
    repo: https://github.com/your-org/infra-template.git
    ref: v1.2.0
```

### .ntplignore（可选）

类 .gitignore 语法，每行一个 glob 模式：

```
logs
*.local
.DS_Store
```

### .ntpl.lock（自动生成）

记录每次同步的版本，用于追溯和状态检查：

```yaml
entries:
  - name: default
    repo: https://github.com/your-org/template.git
    ref: main
    commit: a1b2c3d4e5f6
    time: "2026-04-14T01:00:00Z"
```

## 同步机制

```
模板仓库 (Git)
      ↓  git clone --depth 1 + 删除 .git
本地快照 (.ntpl/template/<name>/)
      ↓  按 include/exclude 过滤
当前项目
```

每次 sync 拉取最新快照，模板缓存中不保留 .git 目录，不干扰项目本身的 git。

## 项目结构

```
cmd/              CLI 命令入口
internal/
  config/         配置解析 + lock 文件 + .ntplignore
  detect/         声明式规则引擎 + 内置规则
  git/            Git clone/export 封装
  sync/           同步、diff、status 核心逻辑
Makefile          构建 & 开发任务
```

## 模板变量替换

使用 `{ntpl:name}` 占位符语法，避免与 Go/JS/Shell/Jinja2 等语言模板冲突。

在 `.ntpl.yaml` 中定义变量：

```yaml
sync:
  vars:
    project_name: my-app
    org: nilorg
    port: "8080"
```

模板文件中使用占位符：

```
module {ntpl:org}/{ntpl:project_name}
listen: :{ntpl:port}
```

sync 时自动将 `{ntpl:key}` 替换为对应值。未定义的变量保留原样不替换。

## Hook 支持

在 sync 前后执行自定义脚本：

```yaml
sync:
  hooks:
    before: ./scripts/backup.sh
    after: ./scripts/gen.sh
```

执行顺序：`before` → 文件同步 → `after`。任一 hook 返回非零退出码则中止。dry-run 模式下不执行 hook。

## 远程配置源

模板仓库根目录可放置 `.ntpl.yaml`，提供默认的 sync 和 vars 配置。同步时自动读取并与本地配置合并，**本地配置始终优先**。

合并规则：
- 本地 `sync.include` 为空时，使用远程 `sync.include`
- 本地 `sync.exclude` 为空时，使用远程 `sync.exclude`
- 远程 `sync.vars` 作为默认值，本地同名 key 覆盖远程
- 远程 `sync.hooks` 不会被合并（安全考虑）

## Pack — 打包项目为模板

将现有项目转换为模板，自动将项目特定值替换为 `{ntpl:key}` 占位符。

```bash
# 自动检测变量
ntpl pack -o ../my-template --suggest

# 手动指定变量
ntpl pack -o ../my-template --var org=nilorg --var project_name=ntpl

# 预览模式
ntpl pack -o ../my-template --suggest --dry-run
```

`--suggest` 使用内置声明式规则自动从 `go.mod`、`package.json`、`pom.xml` 等文件中提取变量，展示给用户确认后执行替换。

## Replace — 替换源仓库的值

同步普通仓库（无 `{ntpl:...}` 占位符）后，将源仓库的值替换为自己的值。

```bash
# 交互式：自动检测源值，逐个输入目标值
ntpl replace --suggest

# 从配置读取
ntpl replace

# 预览
ntpl replace --dry-run
```

配置格式：

```yaml
# .ntpl.yaml
replace:
  rules:
    org: nilorg                    # 简写：auto-detect from, 这是 to
    project_name:                  # 完整写法
      from: "template-project"
      to: "my-app"
```

工作流：`ntpl sync` → `ntpl replace` → 源仓库的值被替换为你的值。

## Replace 排除

`replace` 命令默认跳过 `vendor`、`node_modules` 等第三方依赖目录，避免修改非项目代码。

可通过 `replace.exclude` 配置自定义需要跳过的目录：

```yaml
replace:
  exclude:
    - vendor
    - node_modules
    - third_party
```

未配置时使用默认值（`vendor`、`node_modules`）。`sync` 和 `pack` 命令不受此配置影响。

## 声明式检测规则

检测规则是 YAML 文件，定义从哪个文件用什么正则提取什么变量。`pack` 和 `replace` 的 `--suggest` 模式共享同一套规则。

### 规则加载顺序（后者覆盖前者）

1. 内置规则（embed 在二进制中）
2. 用户规则：`~/.config/ntpl/rules/*.yaml`
3. 项目规则：`.ntpl/rules/*.yaml`

### 规则格式

```yaml
name: go
description: Go project (go.mod)
priority: 10              # 越小越先扫描，默认 50
files:
  - path: go.mod          # 支持 glob
    patterns:
      - regexp: '^module\s+[^/]+/(?P<org>[^/]+)/(?P<project_name>[^/\s]+)'
        description: org and project name
```

使用 Go 正则命名捕获组 `(?P<var_name>...)` 提取变量。贡献新规则只需添加一个 YAML 文件，不需要写 Go 代码。

### 内置规则

| 规则 | 文件 | 提取变量 |
|------|------|----------|
| go | go.mod | org, project_name, module, go_version |
| node | package.json | org, project_name, version, description, author, license |
| python | pyproject.toml, setup.py, setup.cfg | project_name, version, description |
| java | pom.xml, build.gradle, build.gradle.kts | org, project_name, version |
| rust | Cargo.toml | project_name, version, description, license |
| docker | Dockerfile, docker-compose.yaml | port |
| git | .git/config | org, repo |

## AI / LLM 集成

本项目提供 [SKILL.md](SKILL.md)，包含 ntpl 的完整命令参考、配置说明、工作流示例和安全规则。AI 工具或 Agent 可直接读取该文件获取调用 ntpl 所需的全部上下文。

## License

MIT
