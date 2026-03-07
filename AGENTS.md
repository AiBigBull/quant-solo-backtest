# AGENTS.md — quantsolo

本文档为在此仓库中工作的 AI 编码 Agent 提供指引。所有陈述均基于实际源文件中的证据。

---

## 概览

BTC/ETH/SOL 1 倍杠杆合约混合回测系统，使用 Go 实现。系统从 Binance Vision 下载真实月度 K 线数据，对多标的组合进行回测，随机搜索 100,000 组参数，并通过五方委员会投票门控后输出报告。

- 模块名：`quantsolo`（Go 1.23，`go.mod`）
- 主语言：Go
- 无外部 Go 依赖，仅使用标准库

---

## 命令

### 仓库文档中记载的命令（来源：`BACKTEST_SYSTEM.md`）

`BACKTEST_SYSTEM.md` 中记载的完整运行命令：

```bash
go run ./cmd/backtest -iterations 100000 -months 24 -symbols BTCUSDT,ETHUSDT,SOLUSDT
```

该文档还列出了以下可选参数（非独立命令）：

```
-source        Binance 数据源 base URL（默认：https://data.binance.vision）
-data-dir      本地缓存目录（默认：./data）
-interval      K 线周期（默认：1d）
-start-equity  初始资金（默认：100000）
-seed          随机种子，用于复现实验（0 表示随机）
-no-download   跳过网络下载，仅使用 ./data/ 中已缓存的 zip 文件
```

### 推荐的 Go 工具链命令

```bash
# 构建
go build ./cmd/backtest/

# 运行全部测试
go test ./...

# 运行全部测试（详细输出）
go test ./... -v

# 按名称过滤运行单个测试（通用模式）
go test <pkg> -run <Pattern> -v

# 单测示例：委员会通过逻辑
go test ./internal/decision/ -run TestEvaluatePass -v

# 单测示例：月份范围构建
go test ./internal/data/ -run TestBuildMonthlyRange -v

# 静态检查
go vet ./...

# 快速冒烟测试（需要 ./data/ 中已有缓存数据，不联网）
go run ./cmd/backtest -iterations 100 -months 24 -symbols BTCUSDT,ETHUSDT,SOLUSDT -no-download
```

---

## 仓库结构

```
cmd/backtest/main.go          CLI 入口；flag 解析、数据加载、报告写入
internal/backtest/
  types.go                    Bar、Params、Result 结构体定义
  engine.go                   Run() — 组合模拟、权重计算、指标统计
internal/data/
  binance.go                  BinanceClient、BuildMonthlyRange、FilterBarsByTime
  binance_test.go             TestBuildMonthlyRange
internal/decision/
  committee.go                Evaluate() — 五方委员会投票门控
  committee_test.go           TestEvaluatePass、TestEvaluateReject
internal/opt/
  optimizer.go                Optimize() — 随机搜索、压力测试、评分
BACKTEST_SYSTEM.md            系统设计与运行说明（主要参考文档）
MEMORY.md                     不可偏离的用户原始需求
README.md                     Agent 角色映射与 SOP 概览
.gitignore                    忽略：.DS_Store、*.log、*.tmp、data/、/tmp/
```

### 运行时路径与操作目录

- `data/` — 本地 K 线 zip 缓存目录；已被 `.gitignore` 忽略，不纳入 git。运行时由 `-data-dir` 参数指定（默认 `./data`）。
- `reports/` — 报告输出目录；运行时由 `main.go` 通过 `os.MkdirAll("reports", 0o755)` 自动创建，不纳入 git。
- `agents/` — 各岗位 Agent 职责与系统提示词文档；仅供人工参考，不被 Go 代码消费。
- `sop/` — 日常执行、策略生命周期、应急预案 SOP；仅供人工参考，不被 Go 代码消费。
- `templates/` — 需求单、实验记录、复盘模板；仅供人工参考，不被 Go 代码消费。

---

## 代码风格

以下规范从现有源文件推断得出。仓库中无 linter 配置（详见"缺失工具"一节）。

- 使用标准 Go 格式化（`gofmt`），无自定义风格覆盖。
- 包名为小写单词：`backtest`、`data`、`decision`、`opt`。
- 导出类型使用清晰的名词命名：`Bar`、`Params`、`Result`、`BinanceClient`、`RunInput`、`Summary`、`Outcome`、`Vote`。
- 错误使用 `fmt.Errorf("context: %w", err)` 包装并向上传递；`main` 中的致命错误通过本地 `fatalf` 辅助函数写入 stderr 并调用 `os.Exit(1)`。辅助函数命名遵循动词小写风格（如 `fatalf`、`parseSymbols`、`writeJSON`、`printSummary`）。
- `main` 之外无全局可变状态，所有状态通过函数参数或结构体字段传递。
- 新增功能应复用已有结构体和函数模式，例如向 `backtest.RunInput` 传参而非新建全局变量。
- 使用 `any` 而非 `interface{}`（源文件中 `writeJSON` 的签名为 `func writeJSON(path string, v any)`）。
- 文件权限使用八进制字面量：`0o755`（目录）、`0o644`（文件），见 `main.go` 第 125、155 行。
- JSON 字段标签使用 `snake_case`，例如 `json:"generated_at"`、`json:"committee_pass_rate"`、`json:"best_discussed"`。
- 测试与被测代码同包（`package decision`、`package data`），采用白盒风格。
- 测试函数命名遵循 `TestFunctionNameScenario` 模式：`TestEvaluatePass`、`TestEvaluateReject`、`TestBuildMonthlyRange`。
- 测试仅使用标准库 `testing`，无第三方测试框架。
- `math/rand` 显式设置种子以保证可复现；seed 为 0 时回退到 `time.Now().UnixNano()`。

### import 规范

从现有源文件推断，import 分组遵循以下顺序：

```go
import (
    // 第一组：标准库
    "fmt"
    "math"
    "time"

    // 第二组（空行分隔）：内部模块
    "quantsolo/internal/backtest"
    "quantsolo/internal/decision"
)
```

- 标准库在前，内部模块（`quantsolo/internal/...`）在后，两组之间用空行分隔。
- 本仓库无外部第三方依赖，因此不存在第三组。
- `goimports` 不是仓库提供的工具，但可作为本地开发辅助工具自动整理 import 顺序。

## 缺失工具

以下工具经确认不存在，不得凭空引用：

- 无 `Makefile`
- 无 `.github/` 目录（无 CI 工作流，无 GitHub Actions）
- 无 `.cursor/rules/` 目录
- 无 `.cursorrules` 文件
- 无 `.github/copilot-instructions.md`
- 无 `golangci-lint` 配置（`.golangci.yml` / `.golangci.yaml`）
- 无 `go.sum`（无外部依赖）

---

## 测试覆盖空白

- `internal/backtest/engine.go` — 无测试文件
- `internal/opt/optimizer.go` — 无测试文件

为这两个包补充测试时，注意 `engine.Run()` 要求 warmup 后至少 300 条对齐 bar。需构造足够长的合成 `[]backtest.Bar` 切片，满足 warmup 约束：`max(SlowMA, MomentumLookback+1, VolatilityLookback+2)`。

---

## 运行注意事项

- 数据源严格限定为 `https://data.binance.vision`（Binance Futures UM 月度 K 线），不使用合成行情。
- 每个标的经时间范围过滤后至少需要 300 条 bar，否则程序报错退出。
- 杠杆约束：组合权重归一化使 `sum(abs(weight)) <= 1`（1 倍暴露）。
- 默认回测窗口：当前日期往前推一个月为结束月，共 24 个月。
- 默认优化：100,000 次随机参数采样，含压力测试（手续费冲击、滑点冲击、数据扰动变体）。
- 报告输出：`reports/latest_report.json`（JSON 格式）及控制台摘要。
