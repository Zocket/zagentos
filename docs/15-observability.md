# P15: Trace & Observability

## 目标
记录 Agent 执行的完整轨迹，提供可观测性。

## 核心接口
见 `pkg/trace/trace.go`

## 实现要点
- [ ] Span-based tracing
- [ ] 结构化日志
- [ ] Token 使用统计
- [ ] 延迟监控
- [ ] OpenTelemetry 集成
- [ ] Trace 导出（file / stdout / OTLP）
- [ ] 执行链路可视化

## 设计决策
_待填写_

## 建议
从 P3 开始就埋结构化日志，这里是把散落各处的 trace 点收拢和导出。
