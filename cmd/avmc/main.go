package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/John-Robertt/AVMC/internal/app/run"
	"github.com/John-Robertt/AVMC/internal/config"
	"github.com/John-Robertt/AVMC/internal/domain"
	"github.com/John-Robertt/AVMC/internal/infra/fsx"
	"github.com/John-Robertt/AVMC/internal/provider"
	"github.com/John-Robertt/AVMC/internal/provider/javbus"
	"github.com/John-Robertt/AVMC/internal/provider/javdb"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 || isHelp(args[0]) {
		printUsage()
		return
	}

	switch args[0] {
	case "run":
		if code := runCmd(args[1:]); code != 0 {
			os.Exit(code)
		}
	default:
		fmt.Fprintf(os.Stderr, "未知命令：%q\n\n", args[0])
		printUsage()
		os.Exit(2)
	}
}

func runCmd(args []string) int {
	for _, a := range args {
		if isHelp(a) {
			printRunUsage()
			return 0
		}
	}

	ra, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "参数错误：%v\n\n", err)
		printRunUsage()
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取当前目录失败：%v\n", err)
		return 1
	}
	cwdAbs, _ := filepath.Abs(cwd)

	eff, err := config.LoadEffective(cwd, config.CLIArgs{
		Path:        ra.Path,
		Provider:    ra.Provider,
		ProviderSet: ra.ProviderSet,
		Apply:       ra.Apply,
		ApplySet:    ra.ApplySet,
	})
	if err != nil {
		rr := reportForConfigError(cwdAbs, ra, err)
		emitReport(rr)
		return 1
	}

	reg, e := provider.NewRegistry(
		javbus.Provider{},
		javdb.Provider{BaseURL: eff.JavDBBaseURL},
	)
	if e != nil {
		fmt.Fprintf(os.Stderr, "初始化 provider registry 失败：%v\n", e)
		return 1
	}

	progressW, interactive := pickProgressWriter()
	var obs run.Observer
	if interactive {
		obs = newProgressUI(progressW)
	}

	rr := run.ExecuteWithObserver(context.Background(), eff, reg, obs)

	// apply：必须写入 <path>/cache/report.json；dry-run 禁止落盘。
	if eff.Apply {
		if err := writeReportFile(eff.Path, rr); err != nil {
			fmt.Fprintf(os.Stderr, "写入 report.json 失败：%v\n", err)
			emitReport(rr)
			return 1
		}
	}

	emitReport(rr)
	if interactive {
		emitLocations(progressW, eff)
	}
	if rr.Summary.Failed == 0 && rr.Summary.Unmatched == 0 {
		return 0
	}
	return 1
}

type runArgs struct {
	Path        string
	Provider    string
	ProviderSet bool
	Apply       bool
	ApplySet    bool
}

func parseRunArgs(args []string) (runArgs, error) {
	ra := runArgs{}

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--provider":
			if i+1 >= len(args) {
				return runArgs{}, fmt.Errorf("--provider 需要一个值")
			}
			i++
			ra.Provider = args[i]
			ra.ProviderSet = true
		case strings.HasPrefix(a, "--provider="):
			ra.Provider = strings.TrimPrefix(a, "--provider=")
			ra.ProviderSet = true
		case a == "--apply":
			ra.Apply = true
			ra.ApplySet = true
		case strings.HasPrefix(a, "--apply="):
			v := strings.TrimPrefix(a, "--apply=")
			switch v {
			case "true":
				ra.Apply = true
			case "false":
				ra.Apply = false
			default:
				return runArgs{}, fmt.Errorf("--apply 只能是 true 或 false，实际是 %q", v)
			}
			ra.ApplySet = true
		case strings.HasPrefix(a, "-"):
			return runArgs{}, fmt.Errorf("未知参数 %q", a)
		default:
			if ra.Path != "" {
				return runArgs{}, fmt.Errorf("重复的 path：%q 与 %q", ra.Path, a)
			}
			ra.Path = a
		}
	}

	if ra.ProviderSet {
		switch ra.Provider {
		case "javbus", "javdb":
			// ok
		case "":
			return runArgs{}, fmt.Errorf("--provider 不能为空")
		default:
			return runArgs{}, fmt.Errorf("--provider 只能是 javbus 或 javdb，实际是 %q", ra.Provider)
		}
	}

	return ra, nil
}

func isHelp(s string) bool {
	return s == "-h" || s == "--help" || s == "help"
}

func printUsage() {
	fmt.Fprint(os.Stdout, `用法：
  avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]

命令：
  run    运行流程（默认 dry-run）

使用 "avmc run --help" 查看详细说明。
`)
}

func printRunUsage() {
	fmt.Fprint(os.Stdout, `用法：
  avmc run [path] [--provider javbus|javdb] [--apply[=true|false]]

参数：
  --provider  首选 provider：javbus|javdb（未指定则读配置文件；最终默认 javbus）
  --apply     执行落盘与移动（默认 dry-run）；支持 --apply=false 覆盖配置中的 apply=true
  -h, --help  显示帮助
`)
}

func emitReport(rr domain.RunReport) {
	if isTTY(os.Stdout) {
		fmt.Fprintf(os.Stdout, "完成：processed=%d skipped=%d failed=%d unmatched=%d\n",
			rr.Summary.Processed, rr.Summary.Skipped, rr.Summary.Failed, rr.Summary.Unmatched,
		)
		if rr.Summary.Failed > 0 || rr.Summary.Unmatched > 0 {
			for _, it := range rr.Items {
				if it.Status != domain.StatusFailed && it.Status != domain.StatusUnmatched {
					continue
				}
				key := it.Code
				if key == "" && len(it.Files) > 0 {
					// unmatched/config 等合成条目：用首个输入文件路径做定位锚点。
					key = it.Files[0].Src
				}
				if key == "" {
					key = "<unknown>"
				}
				fmt.Fprintf(os.Stderr, "%s %s: %s\n", key, it.ErrorCode, it.ErrorMsg)
			}
		}
		return
	}

	// stdout 非 TTY：stdout 必须且仅输出一个 RunReport JSON（日志/摘要走 stderr）。
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(rr)
	fmt.Fprintf(os.Stderr, "完成：processed=%d skipped=%d failed=%d unmatched=%d\n",
		rr.Summary.Processed, rr.Summary.Skipped, rr.Summary.Failed, rr.Summary.Unmatched,
	)
}

func reportForConfigError(cwdAbs string, ra runArgs, err error) domain.RunReport {
	now := time.Now().UTC()
	rr := domain.RunReport{
		Path:       cwdAbs,
		DryRun:     !(ra.ApplySet && ra.Apply),
		StartedAt:  now,
		FinishedAt: now,
		Items: []domain.ItemResult{{
			Code:              "",
			ProviderRequested: "",
			ProviderUsed:      "",
			Website:           "",
			Status:            domain.StatusFailed,
			ErrorCode:         config.Code(err),
			ErrorMsg:          err.Error(),
			Candidates:        []string{},
			Attempts:          []domain.ProviderAttempt{},
			Files:             []domain.FileResult{},
		}},
	}
	rr.Finalize()
	return rr
}

func writeReportFile(root string, rr domain.RunReport) error {
	b, err := json.MarshalIndent(rr, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return fsx.WriteFileAtomicReplace(filepath.Join(root, "cache"), "report.json", b)
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func pickProgressWriter() (io.Writer, bool) {
	// 进度输出只在交互终端启用；默认走 stderr（不污染 stdout JSON）。
	if isTTY(os.Stderr) {
		return os.Stderr, true
	}
	// 某些环境（例如仅重定向 stderr）下，stdout 仍是 TTY：退化输出到 stdout。
	if isTTY(os.Stdout) {
		return os.Stdout, true
	}
	return nil, false
}

func emitLocations(w io.Writer, eff config.EffectiveConfig) {
	// 这两行用于降低“完成后不知道产物在哪”的摩擦，且不影响 stdout JSON 契约。
	if w == nil {
		return
	}
	if eff.Apply {
		fmt.Fprintf(w, "report: %s\n", filepath.Join(eff.Path, "cache", "report.json"))
	}
	fmt.Fprintf(w, "out: %s\n", filepath.Join(eff.Path, "out"))
}
