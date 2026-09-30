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
