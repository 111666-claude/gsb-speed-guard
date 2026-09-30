# speed-guard

移动速度校验：按时间窗口统计位移，含延迟容忍与连续超限判定，只用 Go 标准库。

```
go test ./...
go run ./cmd/speed-guard --sample latency
go run ./cmd/speed-guard --sample burst
go run ./cmd/speed-guard --sample window
```

## 口径（README 为准）

- **按窗口算最大位移**：窗口内任意两点间位移不得超过
  `MaxSpeed * (dt + ToleranceMs) / 1000`，超限才算一次 strike；高延迟玩家的抖动靠
  `ToleranceMs` 容忍，不许误判。
- **连续超限才判作弊**：strike 连续累计到 `StrikesToBan` 才返回 `true`；
  中间出现一次正常上报就把 strike 清零，第一次超限返回 `false`。
- **窗口要滑出**：超过 `WindowMs` 的样本必须回收，`Samples(now)` 只数窗口内的。
- **查询代价**：不许每次扫描整个样本列表，`Scanned` 不随样本数线性增长。
- **规模**：单服 50 万在线玩家、每秒 20 万次上报，单次摊还 O(1)，内存 O(窗口内样本数)。

## 输出契约（不改格式）

```
banned=false
first=false third=true strikes=3
samples=2
scanned<=4000
```

## 目录

```
guard.go                速度校验
cmd/speed-guard         命令行入口
guard_test.go           go test 用例
```
