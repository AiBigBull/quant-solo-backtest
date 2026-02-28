# Go 回测系统说明（BTC/ETH/SOL 1x 合约混合）

## 功能覆盖

- 交易标的：`BTCUSDT`、`ETHUSDT`、`SOLUSDT`
- 杠杆约束：组合总暴露按 1x 归一化（`sum(abs(weight)) <= 1`）
- 数据来源：仅使用 `https://data.binance.vision/` 的 Binance Futures UM 月度 K 线压缩包
- 回测窗口：默认近 24 个月
- 优化迭代：默认 `100000` 次
- 决策机制：研究、风控、执行、产品、开发“五方委员会”统一通过才算通过

## 目录

- `cmd/backtest/main.go`：CLI 入口
- `internal/data/binance.go`：Binance 真实数据下载/解析
- `internal/backtest/engine.go`：组合回测引擎
- `internal/decision/committee.go`：公司内统一决策门控
- `internal/opt/optimizer.go`：100000 次参数搜索和评分

## 运行

```bash
go run ./cmd/backtest -iterations 100000 -months 24 -symbols BTCUSDT,ETHUSDT,SOLUSDT
```

常用参数：

- `-source`：数据源（默认 `https://data.binance.vision`）
- `-data-dir`：本地缓存目录（默认 `./data`）
- `-interval`：K 线周期（默认 `1d`）
- `-start-equity`：初始资金（默认 `100000`）
- `-seed`：随机种子（用于复现实验）
- `-no-download`：只用本地缓存，不联网下载

## 输出

- 控制台打印：年化收益、Sharpe、最大回撤、月度盈利占比、是否达到目标
- 报告文件：`reports/latest_report.json`

## 目标解释

- “每月整体盈利”在系统中映射为“盈利月占比 >= 50%”（你可在委员会规则里改为更严格阈值）。
- “年化 50% 左右”作为优化目标函数的核心项，不保证一定可达，但会优先搜索靠近该目标的参数。
