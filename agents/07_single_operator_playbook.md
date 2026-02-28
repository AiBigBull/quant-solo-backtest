# 单人操盘编排手册（怎么让多个 Agent 真正协同）

## 场景 1：新增一个 BTC 策略

1. 调用 `Product Manager Agent` 形成需求单（用 `templates/01_requirement_ticket.md`）
2. 调用 `CEO/PM Agent` 做优先级决策与上线门槛定义
3. 调用 `Quant Research Agent` 出回测报告（用 `templates/02_experiment_log.md`）
4. 调用 `Risk Agent` 评估风险边界与上线条件
5. 调用 `Quant Dev Agent` 完成核心交易链路工程化
6. 调用 `Full-Stack Engineer Agent` 实现看板/API/配置管理页面
7. 调用 `Execution Agent` 生成执行计划并评估滑点
8. 调用 `Ops Agent` 设定对账与审计项

上线门槛：
- 回测与仿真证据齐全
- 风控阈值已配置
- 回滚方案可执行

## 场景 1.5：新增后台功能（非策略）

1. `Product Manager Agent` 明确用户故事与验收标准
2. `Full-Stack Engineer Agent` 拆解前端/后端/数据任务
3. `Risk Agent` 审核是否涉及高风险操作（阈值配置、策略开关）
4. `Ops Agent` 补充审计与对账字段
5. `CEO/PM Agent` 批准发布窗口与回滚预案

## 场景 2：实盘出现异常回撤

1. 立即触发 `Risk Agent` 执行 De-risk（减仓/停机）
2. `Execution Agent` 撤单并进入只减仓模式
3. `Ops Agent` 记录事件并对账
4. `Quant Dev Agent` 检查系统异常（延迟、拒单、重复下单）
5. `Quant Research Agent` 判断是否策略失效
6. `CEO/PM Agent` 决策：恢复/降级/下线

## 统一输出约定（建议）

- 每个 Agent 输出都必须包含：结论、证据、风险、下一步
- 决策项必须有截止时间和验收标准
- 任何人工干预必须入日志
