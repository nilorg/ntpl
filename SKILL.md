---
name: ntpl
description: "Sync Git template repositories to existing projects. Use when initializing projects from templates, updating template changes, diffing template vs project, or checking template sync status. Commands: ntpl init, sync, diff, status."
license: MIT
compatibility: "Requires git in PATH and network access to clone repositories"
metadata:
  author: nilorg
  version: "1.0"
---

# ntpl Skill

> 供 AI / LLM Agent 读取的工具参考文档。包含 ntpl 的完整命令、配置、工作流和安全规则。

- **工具名称**: ntpl
- **用途**: 将 Git 模板仓库的更新安全同步到已有项目
- **类型**: CLI 命令行工具
- **语言**: Go（跨平台，无外部依赖）
- **核心命令**: `init`, `sync`, `diff`, `status`

## 前置条件

- 已安装 `ntpl` 命令（`go install github.com/nilorg/ntpl@latest`）
- 已安装 `git`
- 当前工作目录为目标项目根目录

## 命令参考

### init — 初始化配置

在当前目录生成 `.ntpl.yaml` 配置文件。

```bash
ntpl init --repo <git-repo-url>
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--repo` | 是 | 模板 Git 仓库 URL |
| `--force` | 否 | 覆盖已存在的 .ntpl.yaml |

生成的默认配置：
```yaml
templates:
  - name: default
    repo: <url>
    ref: main
sync:
  include: []
  exclude: []
```

### sync — 同步模板到项目

从配置的模板仓库拉取最新文件并同步到当前项目。

```bash
ntpl sync
ntpl sync --dry-run
ntpl sync -i
```

| 参数 | 说明 |
|------|------|
| `--dry-run` | 只预览变更，不修改任何文件 |
| `-i`, `--interactive` | 交互式同步，逐文件询问 y/n/q |

同步行为：
- 只覆盖 `include` 范围内且不在 `exclude` / `.ntplignore` 中的文件
- 不会删除项目中有但模板中没有的文件
- 同步完成后自动更新 `.ntpl.lock` 记录 commit hash

### diff — 查看差异

拉取最新模板并显示与项目的文件级差异。

```bash
ntpl diff
```

输出分类：
- `only in template:` — 模板有但项目没有的文件
- `only in project:` — 项目有但模板没有的文件
- `modified:` — 两端都有但内容不同的文件（附 unified diff）

### status — 查看同步状态

读取 `.ntpl.lock` 并与远程仓库对比，显示是否有更新。

```bash
ntpl status
```

输出字段：repo、ref、commit、synced at、status（`up to date` 或 `update available`）。

### pack — 打包项目为模板

将现有项目转换为模板，自动将项目特定值替换为 `{ntpl:key}` 占位符。

```bash
ntpl pack -o <output-dir> --suggest          # 自动检测变量
ntpl pack -o <output-dir> --var org=nilorg   # 手动指定
ntpl pack -o <output-dir> --suggest --dry-run
```

| 参数 | 说明 |
|------|------|
| `-o`, `--output` | 输出目录（必填） |
| `--var` | 手动指定变量（key=value，可重复） |
| `--suggest` | 使用声明式规则自动检测变量 |
| `--dry-run` | 预览模式 |

输出目录会自动生成 `.ntpl.yaml`（包含 vars 默认值）。

### replace — 替换源值

同步普通仓库后，将源仓库的值替换为自己的值。

```bash
ntpl replace                  # 从 .ntpl.yaml 的 replace 配置读取
ntpl replace --suggest        # 交互式检测并替换
ntpl replace --dry-run
```

| 参数 | 说明 |
|------|------|
| `--suggest` | 交互式：自动检测源值，逐个输入目标值 |
| `--dry-run` | 预览模式 |

replace 配置格式：
```yaml
replace:
  rules:
    org: nilorg                   # 简写：auto-detect from
    project_name:                 # 完整写法
      from: "template-project"
      to: "my-app"
```

## 配置文件

### .ntpl.yaml（核心配置）

```yaml
templates:                       # 模板源列表
  - name: default                # 模板名（唯一标识）
    repo: <git-url>              # Git 仓库 URL
    ref: main                    # 分支名、tag 或 commit hash

sync:
  include:                       # 要同步的目录/文件（空 = 全部同步）
    - src
    - configs
  exclude:                       # 排除的目录/文件
    - .env
    - config.local.yaml
  vars:                          # 模板变量，替换 {ntpl:key} 占位符
    project_name: my-app
    org: nilorg
  hooks:                         # sync 前后执行脚本
    before: ./scripts/backup.sh
    after: ./scripts/gen.sh

replace:
  exclude:                       # replace 跳过的目录（默认 vendor, node_modules）
    - vendor
    - node_modules
  rules:                         # 替换规则
    org: nilorg
    project_name:
      from: "template-project"
      to: "my-app"
```

支持多模板源：
```yaml
templates:
  - name: backend
    repo: https://github.com/org/backend-tpl.git
    ref: main
  - name: infra
    repo: https://github.com/org/infra-tpl.git
    ref: v1.2.0
```

### .ntplignore（可选，类 .gitignore 语法）

每行一个 glob 模式，匹配的文件不会被同步或 diff：
```
logs
*.local
.DS_Store
```

### .ntpl.lock（自动生成，勿手动编辑）

记录每次同步的精确版本：
```yaml
entries:
  - name: default
    repo: <url>
    ref: main
    commit: a1b2c3d4e5f6
    time: "2026-04-14T01:00:00Z"
```

## 典型工作流

### 场景 1：新项目初始化并同步模板

```bash
mkdir my-project && cd my-project
git init
ntpl init --repo https://github.com/org/template.git
# 编辑 .ntpl.yaml 设置 include/exclude（可选）
ntpl sync
```

### 场景 2：已有项目更新模板

```bash
cd my-project
ntpl status                     # 查看是否有更新
ntpl diff                       # 查看具体差异
ntpl sync --dry-run             # 预览变更
ntpl sync                       # 确认后同步
```

### 场景 3：谨慎同步（交互模式）

```bash
ntpl sync -i
# 每个文件会提示: create/update <file>? [y/n/q]
# y = 同步此文件, n = 跳过, q = 中止
```

### 场景 4：锁定特定版本

在 `.ntpl.yaml` 中将 `ref` 设为 tag 或 commit hash：
```yaml
templates:
  - name: default
    repo: https://github.com/org/template.git
    ref: v1.0.0           # 或 commit hash: a1b2c3d
```

### 场景 5：多模板源组合

```yaml
templates:
  - name: backend
    repo: https://github.com/org/backend-tpl.git
    ref: main
  - name: ci
    repo: https://github.com/org/ci-tpl.git
    ref: main
sync:
  include:
    - src
    - .github
```

## 安全规则

- 以下文件始终被排除，不会被同步覆盖：`.ntpl/`、`.ntpl.yaml`、`.ntpl.lock`、`.ntplignore`
- `exclude` 和 `.ntplignore` 匹配的文件不会被读取或写入
- sync 不执行删除操作——只创建和更新文件
- `--dry-run` 只输出变更列表，不产生任何副作用

## 文件结构约定

ntpl 在项目目录中管理以下文件和目录：

```
.ntpl.yaml           用户编辑的配置（需纳入版本控制）
.ntplignore          用户编辑的排除规则（需纳入版本控制）
.ntpl.lock           自动生成的版本锁（建议纳入版本控制）
.ntpl/               缓存目录（不纳入版本控制）
  template/
    <name>/          每个模板源的文件快照
```

## 注意事项

- ntpl 依赖 git 命令，确保 `git` 在 PATH 中
- 每次 sync/diff 都会重新拉取模板仓库（浅克隆），需要网络连接
- ref 支持 branch、tag 和完整 commit hash（7-40 位十六进制）
- include 为空时同步模板仓库的全部文件
- glob 匹配规则同 Go `filepath.Match`：`*` 匹配非分隔符字符，`?` 匹配单个字符
- `{ntpl:key}` 变量替换在文件写入时执行，未定义的变量保留原样
- hooks 仅从本地 `.ntpl.yaml` 的 `sync.hooks` 读取，远程模板的 hooks 不会被合并（安全考虑）
- 远程配置源：模板仓库根目录的 `.ntpl.yaml` 提供 sync 默认值，本地配置优先
- pack/replace 的 `--suggest` 使用声明式规则自动检测变量，规则可自定义扩展
- 检测规则加载顺序：内置 → `~/.config/ntpl/rules/` → `.ntpl/rules/`，同名后者覆盖
- replace 按值长度降序替换，避免短值误替换长值的子串
- replace 默认跳过 `vendor`、`node_modules` 等依赖目录，可通过 `replace.exclude` 自定义
