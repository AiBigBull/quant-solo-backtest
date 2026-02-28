# 量化一人公司 Agent 系统（BTC 自动量化）

这套文件用于把“小团队岗位分工”映射成“单人 + 多 Agent 协同”，目标是：
- 一个人同时扮演产品、研究、开发、交易、风控、运营角色
- 用标准化提示词和 SOP 降低拍脑袋决策
- 优先保证资金安全、系统稳定和可追溯

## 目录

- `agents/`：各岗位 Agent 的职责、输入输出、可直接复制的系统提示词
- `sop/`：日常执行、策略生命周期、应急预案
- `templates/`：需求单、实验记录、复盘模板

## 角色映射（单人公司）

1. **CEO/PM Orchestrator Agent**：统一优先级、排期、决策留痕
2. **Quant Research Agent**：策略研究、回测、参数稳健性
3. **Quant Dev Agent**：实盘系统开发、监控、工程质量
4. **Execution Agent**：下单执行、滑点和成交质量优化
5. **Risk Agent**：仓位/杠杆/回撤/熔断控制
6. **Ops & Compliance Agent**：对账、报表、审计与合规清单
7. **Data Agent**：行情/订单簿/资金费率/链上数据管理
8. **Product Manager Agent**：需求定义、优先级管理、版本规划
9. **Full-Stack Engineer Agent**：前后端系统实现、看板与接口交付

## 建议运行方式

1. 每天开始先跑 `sop/01_daily_checklist.md`
2. 新策略按 `sop/02_strategy_lifecycle.md` 执行
3. 出现异常时按 `sop/03_incident_runbook.md` 响应
4. 所有任务必须落地到 `templates/` 的记录模板

## 最小执行原则

- 不允许无风控上线
- 不允许未回测/未仿真直接实盘
- 不允许无法追溯的人工临时改配置
- 任何临时人工干预都要记录原因、时间、影响
