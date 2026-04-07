# ado — Azure DevOps CLI

[繁體中文](#繁體中文) | [English](#english)

---

## 繁體中文

輕量級 Azure DevOps CLI 工具，採用 CQRS + MediatR 模式設計。

### 架構

```
cmd/                    # Cobra CLI 指令
internal/
  cqrs/                 # Mediator — Request / Handler / PipelineBehavior
  behaviors/            # Behavior pipeline (middleware)
  features/             # 每個 use case 獨立一個 handler
    query/              #   └─ GetQuery: 依 query ID 取得工作項目
  api/                  # Azure DevOps REST API client
  config/               # 環境變數設定
```

**設計原則：**
- **CQRS** — Query 與 Command 分離，每個 handler 獨立
- **Behavior Pipeline** — 類似 MediatR 的 middleware chain（Logging、Retry …）
- **Streaming** — handler 透過 `io.Writer` 逐筆輸出，不等全部完成才顯示

### 安裝與設定

```bash
go build -o ado .
```

複製 `.env.example` 為 `.env` 並填入：

```
ADO_ORG=https://dev.azure.com/your-org
ADO_PROJECT=your-project
ADO_PAT=your-personal-access-token
ADO_QUERY_ID=your-saved-query-id
```

### 使用方式

```bash
# 使用 .env 中預設的 query ID
./ado query

# 指定 query ID
./ado query -i <query-id>
```

---

## English

Lightweight Azure DevOps CLI tool built with a CQRS + MediatR-style architecture.

### Architecture

```
cmd/                    # Cobra CLI commands
internal/
  cqrs/                 # Mediator — Request / Handler / PipelineBehavior
  behaviors/            # Behavior pipeline (middleware)
  features/             # One handler per use case
    query/              #   └─ GetQuery: fetch work items by query ID
  api/                  # Azure DevOps REST API client
  config/               # Environment-based configuration
```

**Design principles:**
- **CQRS** — Queries and Commands are separate; each handler is independent
- **Behavior Pipeline** — MediatR-style middleware chain (Logging, Retry, …)
- **Streaming** — Handlers write to `io.Writer`, streaming results as they arrive

### Setup

```bash
go build -o ado .
```

Copy `.env.example` to `.env` and fill in:

```
ADO_ORG=https://dev.azure.com/your-org
ADO_PROJECT=your-project
ADO_PAT=your-personal-access-token
ADO_QUERY_ID=your-saved-query-id
```

### Usage

```bash
# Use default query ID from .env
./ado query

# Specify a query ID
./ado query -i <query-id>
```
