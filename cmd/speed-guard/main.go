// Command speed-guard 跑速度校验样例。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"example.com/speedguard"
)

// Run 执行一次命令行调用，返回退出码。
func Run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("speed-guard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	sample := flags.String("sample", "latency", "latency / burst / window / scan")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	switch *sample {
	case "latency":
		// 高延迟玩家：300ms 走了 4 米，含延迟容忍后并不超限。
		guard := &speedguard.Guard{}
		guard.Report(speedguard.Sample{AtMs: 0, X: 0})
		banned := guard.Report(speedguard.Sample{AtMs: 300, X: 4})
		fmt.Fprintf(stdout, "banned=%v\n", banned)
	case "burst":
		// 连续三次超限：口径是第三次才判作弊。
		guard := &speedguard.Guard{}
		guard.Report(speedguard.Sample{AtMs: 0, X: 0})
		results := make([]bool, 0, 3)
		for index := 1; index <= 3; index++ {
			results = append(results, guard.Report(speedguard.Sample{AtMs: int64(index * 100), X: float64(index * 5)}))
		}
		fmt.Fprintf(stdout, "first=%v third=%v strikes=%d\n", results[0], results[2], guard.Strikes())
	case "window":
		// 十秒一秒一个点，窗口只有一秒。
		guard := &speedguard.Guard{}
		for index := 0; index <= 10; index++ {
			guard.Report(speedguard.Sample{AtMs: int64(index * 1000), X: float64(index)})
		}
		fmt.Fprintf(stdout, "samples=%d\n", guard.Samples(10000))
	case "scan":
		guard := &speedguard.Guard{}
		for index := 0; index < 2000; index++ {
			guard.Report(speedguard.Sample{AtMs: int64(index * 500), X: float64(index)})
		}
		fmt.Fprintf(stdout, "scanned=%d\n", guard.Scanned())
	default:
		fmt.Fprintln(stderr, "需要 --sample latency|burst|window|scan")
		return 2
	}
	return 0
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
