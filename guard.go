// Package speedguard 是移动速度校验：按时间窗口统计位移，含延迟容忍与连续超限判定。
// 缺陷：只看相邻两点平均速度、忽略延迟容忍、一次超限就判作弊、窗口外样本不回收、查询全扫。
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

// Guard 是速度校验器。
type Guard struct {
	samples []Sample
	strikes int
	scanned int
}

// Report 上报一次位置，返回是否判定为作弊。
// 缺陷：只拿最后两点算平均速度（不看窗口内最大位移）、完全不看 ToleranceMs、
// 一次超限就返回 true、窗口外样本从不回收、每次扫描整个样本列表。
func (g *Guard) Report(step Sample) bool {
	g.samples = append(g.samples, step)
	g.scanned += len(g.samples)
	if len(g.samples) < 2 {
		return false
	}
	last := g.samples[len(g.samples)-1]
	prev := g.samples[len(g.samples)-2]
	seconds := float64(last.AtMs-prev.AtMs) / 1000.0
	if seconds <= 0 {
		return false
	}
	if distance(prev, last)/seconds > MaxSpeed {
		g.strikes++
		return true
	}
	return false
}

// Strikes 是累计超限次数。
func (g *Guard) Strikes() int { return g.strikes }

// Samples 是窗口内的样本数。缺陷：窗口外的样本也留着。
func (g *Guard) Samples(nowMs int64) int { return len(g.samples) }

// Scanned 是累计扫描样本数（规模观测）。
func (g *Guard) Scanned() int { return g.scanned }

func distance(a, b Sample) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}
