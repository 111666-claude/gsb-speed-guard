package speedguard

import "testing"

func TestFirstReportIsNotBanned(t *testing.T) {
	if (&Guard{}).Report(Sample{AtMs: 0}) {
		t.Fatal("第一次上报不该判作弊")
	}
}

func TestSlowWalkIsNotBanned(t *testing.T) {
	guard := &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	if guard.Report(Sample{AtMs: 1000, X: 1}) {
		t.Fatal("一秒走一米不该判作弊")
	}
}

func TestStrikesStartAtZero(t *testing.T) {
	if (&Guard{}).Strikes() != 0 {
		t.Fatal("初始超限次数应该是 0")
	}
}

func TestConstants(t *testing.T) {
	if WindowMs != 1000 || MaxSpeed != 10 || ToleranceMs != 200 || StrikesToBan != 3 {
		t.Fatal("速度校验常量被改了")
	}
}

func TestLatencyTolerance(t *testing.T) {
	guard := &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	if guard.Report(Sample{AtMs: 300, X: 4}) {
		t.Fatal("高延迟抖动不该判作弊")
	}
	if guard.Strikes() != 0 {
		t.Fatal("容忍内的上报不该累计 strike")
	}
}

func TestConsecutiveStrikes(t *testing.T) {
	guard := &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	if guard.Report(Sample{AtMs: 100, X: 5}) {
		t.Fatal("第一次超限不该判作弊")
	}
	if guard.Report(Sample{AtMs: 200, X: 10}) {
		t.Fatal("第二次超限不该判作弊")
	}
	if !guard.Report(Sample{AtMs: 300, X: 15}) {
		t.Fatal("连续第三次超限该判作弊")
	}
	if guard.Strikes() != 3 {
		t.Fatalf("strikes 应该是 3，得到 %d", guard.Strikes())
	}
}

func TestNormalReportResetsStrikes(t *testing.T) {
	guard := &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	guard.Report(Sample{AtMs: 100, X: 5})
	guard.Report(Sample{AtMs: 200, X: 10})
	if guard.Strikes() != 2 {
		t.Fatalf("strikes 应该是 2，得到 %d", guard.Strikes())
	}
	if guard.Report(Sample{AtMs: 1000, X: 10}) {
		t.Fatal("正常上报不该判作弊")
	}
	if guard.Strikes() != 0 {
		t.Fatalf("正常上报后 strikes 该清零，得到 %d", guard.Strikes())
	}
	guard.Report(Sample{AtMs: 1100, X: 15})
	guard.Report(Sample{AtMs: 1200, X: 20})
	if !guard.Report(Sample{AtMs: 1300, X: 25}) {
		t.Fatal("清零后重新累计，连续第三次该判作弊")
	}
}

func TestWindowEviction(t *testing.T) {
	guard := &Guard{}
	for index := 0; index <= 10; index++ {
		guard.Report(Sample{AtMs: int64(index * 1000), X: float64(index)})
	}
	if got := guard.Samples(10000); got != 2 {
		t.Fatalf("窗口内该剩 2 个样本，得到 %d", got)
	}
	if got := guard.Samples(20000); got != 0 {
		t.Fatalf("全部滑出后该剩 0 个样本，得到 %d", got)
	}
}

func TestScanCostIsBounded(t *testing.T) {
	guard := &Guard{}
	for index := 0; index < 2000; index++ {
		guard.Report(Sample{AtMs: int64(index * 500), X: float64(index)})
	}
	if scanned := guard.Scanned(); scanned > 4000 {
		t.Fatalf("scanned 不该随样本数线性增长，得到 %d", scanned)
	}
}

func TestDeterministicReplay(t *testing.T) {
	sequence := []Sample{
		{AtMs: 0, X: 0}, {AtMs: 100, X: 5}, {AtMs: 250, X: 1},
		{AtMs: 400, X: 9}, {AtMs: 1500, X: 9}, {AtMs: 1600, X: 14},
		{AtMs: 1700, X: 19}, {AtMs: 1800, X: 24},
	}
	run := func() []bool {
		guard := &Guard{}
		results := make([]bool, 0, len(sequence))
		for _, step := range sequence {
			results = append(results, guard.Report(step))
		}
		return results
	}
	first, second := run(), run()
	if len(first) != len(second) {
		t.Fatal("重复执行结果长度不一致")
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("第 %d 次判定在重复执行间不一致", index)
		}
	}
}

// referenceGuard 是朴素参考实现：窗口最大位移 + 容忍量 + 连续计数，每次全量重算。
type referenceGuard struct {
	samples []Sample
	strikes int
}

func (g *referenceGuard) report(step Sample) bool {
	kept := g.samples[:0]
	for _, other := range g.samples {
		if step.AtMs-other.AtMs <= WindowMs {
			kept = append(kept, other)
		}
	}
	g.samples = kept

	violation := false
	for _, other := range g.samples {
		dtSeconds := float64(step.AtMs-other.AtMs) / 1000.0
		limit := MaxSpeed * (dtSeconds + ToleranceMs/1000.0)
		if distance(other, step) > limit {
			violation = true
		}
	}
	g.samples = append(g.samples, step)

	if violation {
		g.strikes++
		return g.strikes >= StrikesToBan
	}
	g.strikes = 0
	return false
}

func (g *referenceGuard) samplesInWindow(nowMs int64) int {
	count := 0
	for _, other := range g.samples {
		if nowMs-other.AtMs <= WindowMs {
			count++
		}
	}
	return count
}

// rand 是确定性伪随机源（线性同余），只用标准库可复现。
type rand struct{ state uint64 }

func (r *rand) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 11
}

func (r *rand) float64() float64 { return float64(r.next()%1_000_000) / 1_000_000 }

func TestDifferentialAgainstReference(t *testing.T) {
	rng := &rand{state: 42}
	for trial := 0; trial < 200; trial++ {
		guard := &Guard{}
		reference := &referenceGuard{}
		now := int64(0)
		length := 20 + int(rng.next()%30)
		for step := 0; step < length; step++ {
			now += int64(rng.next() % 400)
			var x, y float64
			if rng.next()%5 == 0 {
				// 真实瞬移：直接跳几十米。
				x = rng.float64() * 80
				y = rng.float64() * 80
			} else {
				// 高延迟抖动：小步移动。
				x = rng.float64() * 6
				y = rng.float64() * 6
			}
			sample := Sample{AtMs: now, X: x, Y: y}
			got, want := guard.Report(sample), reference.report(sample)
			if got != want {
				t.Fatalf("trial %d step %d: 判定不一致 got=%v want=%v", trial, step, got, want)
			}
			if guard.Strikes() != reference.strikes {
				t.Fatalf("trial %d step %d: strikes 不一致 got=%d want=%d", trial, step, guard.Strikes(), reference.strikes)
			}
			if got, want := guard.Samples(now), reference.samplesInWindow(now); got != want {
				t.Fatalf("trial %d step %d: 窗口样本数不一致 got=%d want=%d", trial, step, got, want)
			}
		}
	}
}
