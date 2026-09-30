package speedguard

import (
	"math/rand"
	"testing"
)

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
	// 300ms 走 4 米：上限是 10*(300+200)/1000=5 米，不应判超限。
	guard := &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	if guard.Report(Sample{AtMs: 300, X: 4}) {
		t.Fatal("延迟容忍内的抖动不该判作弊")
	}
	if guard.Strikes() != 0 {
		t.Fatal("正常上报不该增加 strike")
	}
	// 间歇性高延迟抖动：相邻两点 4 米/300ms（裸速 13.3m/s，靠容忍放行），
	// 两次抖动间隔超过窗口，窗口内不会累计位移。
	guard = &Guard{}
	jitterTimes := []int64{0, 300, 1400, 1700, 2800, 3100}
	for index, at := range jitterTimes {
		if guard.Report(Sample{AtMs: at, X: float64((index / 2) * 4)}) {
			t.Fatal("容忍内的持续抖动不该判作弊")
		}
	}
	if guard.Strikes() != 0 {
		t.Fatal("容忍内的抖动不该增加 strike")
	}
	// 超出容忍：300ms 走 6 米 > 5 米，算一次 strike，但第一次不封号。
	guard = &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	if banned := guard.Report(Sample{AtMs: 300, X: 6}); banned || guard.Strikes() != 1 {
		t.Fatalf("首次超响应为 banned=false, strikes=1，实际 banned=%v strikes=%d", banned, guard.Strikes())
	}
}

func TestConsecutiveStrikes(t *testing.T) {
	// 每 100ms 瞬移 5 米（上限 3 米），连续三次才判。
	guard := &Guard{}
	guard.Report(Sample{AtMs: 0, X: 0})
	var results []bool
	for index := 1; index <= 3; index++ {
		results = append(results, guard.Report(Sample{AtMs: int64(index * 100), X: float64(index * 5)}))
	}
	if results[0] || !results[2] || guard.Strikes() != 3 {
		t.Fatalf("口径应为 first=false third=true strikes=3，实际 first=%v third=%v strikes=%d",
			results[0], results[2], guard.Strikes())
	}
	// 窗口滑出后来一次正常上报，计数清零，需要重新连续三次。
	// （瞬移后原地不动仍与窗口内早先点构成超限，必须等旧点回收。）
	guard.Report(Sample{AtMs: 1400, X: 15})
	if guard.Strikes() != 0 {
		t.Fatal("正常上报应清零 strike")
	}
	for index := 0; index < StrikesToBan-1; index++ {
		if guard.Report(Sample{AtMs: int64(1500 + index*100), X: float64(20 + index*5)}) {
			t.Fatal("计数清零后重新累计，前两次都不该判")
		}
	}
	if guard.Strikes() != StrikesToBan-1 {
		t.Fatalf("重新累计后 strikes 应为 %d", StrikesToBan-1)
	}
}

func TestWindowEviction(t *testing.T) {
	guard := &Guard{}
	for index := 0; index <= 10; index++ {
		guard.Report(Sample{AtMs: int64(index * 1000), X: float64(index)})
	}
	if samples := guard.Samples(10000); samples != 2 {
		t.Fatalf("1 秒窗口在 10000ms 时只应有 2 个样本，实际 %d", samples)
	}
	// 底层存储不能随总上报数线性增长（O(窗口内样本数)）。
	if len(guard.samples) > 8 {
		t.Fatalf("超窗样本未回收，底层存储 %d 个", len(guard.samples))
	}
	if Samples := (&Guard{}).Samples(12345); Samples != 0 {
		t.Fatalf("空校验器窗口内样本数应为 0，实际 %d", Samples)
	}
}

func TestScanCost(t *testing.T) {
	guard := &Guard{}
	for index := 0; index < 2000; index++ {
		guard.Report(Sample{AtMs: int64(index * 500), X: float64(index)})
	}
	if scanned := guard.Scanned(); scanned > 4000 {
		t.Fatalf("2000 次上报扫描数应在 4000 以内，实际 %d", scanned)
	}
	// 再翻一倍上报，扫描量仍应与上报数（而非总样本数）同阶：
	// 每报只与窗口内（至多 3 个）样本比较。
	for index := 2000; index < 4000; index++ {
		guard.Report(Sample{AtMs: int64(index * 500), X: float64(index)})
	}
	if scanned := guard.Scanned(); scanned > 12000 {
		t.Fatalf("扫描量随历史长度超线性增长，实际 %d", scanned)
	}
}

func TestReferenceParity(t *testing.T) {
	const sequences = 200
	for seq := 0; seq < sequences; seq++ {
		rng := rand.New(rand.NewSource(int64(seq)))
		length := 1 + rng.Intn(60)
		steps := make([]Sample, length)
		now := int64(0)
		x, y := 0.0, 0.0
		for index := range steps {
			now += int64(20 + rng.Intn(481)) // 20~500ms 抖动，模拟高延迟
			if rng.Intn(10) == 0 {
				// 真实瞬移：大幅跳变
				x += 40 + rng.Float64()*60
				if rng.Intn(2) == 0 {
					x = -x
				}
			} else {
				// 正常移动：0~6 米/段（多数在容忍内，边界附近随机）
				x += (rng.Float64()*2 - 1) * 6
			}
			y += (rng.Float64()*2 - 1) * 6
			steps[index] = Sample{AtMs: now, X: x, Y: y}
		}

		guard := &Guard{}
		reference := &referenceGuard{}
		var replayed []bool
		for index, step := range steps {
			got := guard.Report(step)
			want := reference.report(step)
			if got != want || guard.Strikes() != reference.strikes {
				t.Fatalf("序列 %d 第 %d 报不一致：实现 banned=%v strikes=%d，参考 banned=%v strikes=%d",
					seq, index, got, guard.Strikes(), want, reference.strikes)
			}
			replayed = append(replayed, got)
		}

		// 不变量：同一串上报序列重复执行结果相同。
		again := &Guard{}
		for index, step := range steps {
			if again.Report(step) != replayed[index] {
				t.Fatalf("序列 %d 第 %d 报重放结果不一致", seq, index)
			}
		}
	}
}

// referenceGuard 是「窗口最大位移 + 容忍量 + 连续计数」的直白参考实现：
// 每报遍历全量历史筛出窗口样本，与新样本逐对比较，O(n^2) 不追求性能。
type referenceGuard struct {
	history []Sample
	strikes int
}

func (r *referenceGuard) report(step Sample) bool {
	r.history = append(r.history, step)
	exceeded := false
	for _, old := range r.history[:len(r.history)-1] {
		if step.AtMs-old.AtMs > WindowMs {
			continue
		}
		if exceeds(old, step) {
			exceeded = true
		}
	}
	if exceeded {
		r.strikes++
	} else {
		r.strikes = 0
	}
	return r.strikes >= StrikesToBan
}
