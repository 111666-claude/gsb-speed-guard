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

// Sample 是一次位置上报。
type Sample struct {
	AtMs int64
	X, Y float64
}

// Guard 是速度校验器。样本按到达顺序入队，超窗样本从队首回收，
// 单次上报摊还 O(1)，内存 O(窗口内样本数)。
type Guard struct {
	samples []Sample
	head    int
	strikes int
	scanned int
}

// Report 上报一次位置，返回是否判定为作弊。
// 口径：先回收超窗样本，再比较新样本与窗口内每个样本的位移，
// 超过 MaxSpeed*(dt+ToleranceMs)/1000 记一次超限；连续超限达到
// StrikesToBan 才返回 true，中间正常一次就把计数清零。
func (g *Guard) Report(step Sample) bool {
	g.evict(step.AtMs)
	exceeded := false
	for index := g.head; index < len(g.samples); index++ {
		g.scanned++
		if exceeds(g.samples[index], step) {
			exceeded = true
		}
	}
	g.samples = append(g.samples, step)
	if exceeded {
		g.strikes++
	} else {
		g.strikes = 0
	}
	return g.strikes >= StrikesToBan
}

// Strikes 是累计超限次数。
func (g *Guard) Strikes() int { return g.strikes }

// Samples 是窗口内的样本数，超窗样本先回收再计数。
func (g *Guard) Samples(nowMs int64) int {
	g.evict(nowMs)
	return len(g.samples) - g.head
}

// Scanned 是累计扫描样本数（规模观测）。
func (g *Guard) Scanned() int { return g.scanned }

// evict 回收相对 nowMs 超过 WindowMs 的队首样本，必要时压缩底层数组，
// 摊还 O(1)。
func (g *Guard) evict(nowMs int64) {
	for g.head < len(g.samples) && nowMs-g.samples[g.head].AtMs > WindowMs {
		g.head++
	}
	if g.head > 0 && g.head*2 >= len(g.samples) {
		kept := copy(g.samples, g.samples[g.head:])
		g.samples = g.samples[:kept]
		g.head = 0
	}
}

// exceeds 判断两点间位移是否超过 MaxSpeed*(dt+ToleranceMs)/1000。
func exceeds(a, b Sample) bool {
	dt := a.AtMs - b.AtMs
	if dt < 0 {
		dt = -dt
	}
	return distance(a, b) > MaxSpeed*float64(dt+ToleranceMs)/1000.0
}

func distance(a, b Sample) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}
