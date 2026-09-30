// Package speedguard 是移动速度校验：按时间窗口统计位移，含延迟容忍与连续超限判定。
package speedguard

import "math"

// WindowMs 是校验窗口。
const WindowMs = 1000

// MaxSpeed 是允许的最大速度（米每秒）。
const MaxSpeed = 10.0

// ToleranceMs 是延迟容忍（毫秒）。
const ToleranceMs = 200

// StrikesToBan 是判定作弊所需的连续超限次数。
const StrikesToBan = 3

// compactHead 是环形队列压缩前允许积压的已回收样本数下限。
const compactHead = 64

// Sample 是一次位置上报。
type Sample struct {
	AtMs int64
	X, Y float64
}

// Guard 是速度校验器。样本以队列形式保存，head 之前的样本已滑出窗口。
type Guard struct {
	samples []Sample
	head    int
	strikes int
	scanned int
}

// Report 上报一次位置，返回是否判定为作弊。
// 窗口内任意一点与本次上报的位移超过 MaxSpeed*(dt+ToleranceMs)/1000 算一次 strike；
// strike 必须连续累计到 StrikesToBan 才返回 true，中间一次正常即清零。
func (g *Guard) Report(step Sample) bool {
	g.evict(step.AtMs)

	violation := false
	for index := g.head; index < len(g.samples); index++ {
		g.scanned++
		other := g.samples[index]
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

// Strikes 是当前连续超限次数。
func (g *Guard) Strikes() int { return g.strikes }

// Samples 是窗口内（与 nowMs 相差不超过 WindowMs）的样本数。
func (g *Guard) Samples(nowMs int64) int {
	g.evict(nowMs)
	return len(g.samples) - g.head
}

// Scanned 是累计配对检查的样本数（规模观测），只与窗口内样本数有关。
func (g *Guard) Scanned() int { return g.scanned }

// evict 回收早于 nowMs-WindowMs 的样本，摊还 O(1)。
func (g *Guard) evict(nowMs int64) {
	cutoff := nowMs - WindowMs
	for g.head < len(g.samples) && g.samples[g.head].AtMs < cutoff {
		g.head++
	}
	if g.head >= compactHead && g.head*2 >= len(g.samples) {
		alive := len(g.samples) - g.head
		copy(g.samples, g.samples[g.head:])
		g.samples = g.samples[:alive]
		g.head = 0
	}
}

func distance(a, b Sample) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}
