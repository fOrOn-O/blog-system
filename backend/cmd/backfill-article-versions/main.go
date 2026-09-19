package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"blog-system/internal/config"
	"blog-system/internal/database"
	"blog-system/internal/migration"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("backfill-article-versions", flag.ContinueOnError)
	flags.SetOutput(errOut)
	apply := flags.Bool("apply", false, "执行回填；须先备份、停止业务写入并确认是历史遗留数据")
	dryRun := flags.Bool("dry-run", false, "只检查并输出计划（默认行为，不写数据库）")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || (*apply && *dryRun) {
		fmt.Fprintln(errOut, "参数无效：--apply 与 --dry-run 不可同时使用，且不接受位置参数")
		return 2
	}
	// 两个环境变量都必须明确提供，绝不回退到 config.yml 或默认本地数据库。
	cfg := config.Database{Driver: strings.ToLower(strings.TrimSpace(os.Getenv("BLOG_DATABASE_DRIVER"))), DSN: os.Getenv("BLOG_DATABASE_DSN")}
	if cfg.Driver == "" || strings.TrimSpace(cfg.DSN) == "" {
		fmt.Fprintln(errOut, "必须显式设置 BLOG_DATABASE_DRIVER 和 BLOG_DATABASE_DSN")
		return 2
	}
	db, err := database.OpenMigrationDatabase(cfg, *apply)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	encoder := json.NewEncoder(out)
	mode := "dry-run"
	if *apply {
		mode = "apply"
	}
	_ = encoder.Encode(map[string]string{"mode": mode, "driver": cfg.Driver})
	var outputErr error
	report, err := migration.Run(ctx, db, *apply, func(result migration.Result) {
		if outputErr == nil {
			outputErr = encoder.Encode(result)
		}
	})
	if e := encoder.Encode(report); e != nil {
		outputErr = e
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if outputErr != nil {
		fmt.Fprintln(errOut, "迁移报告输出失败；请重新 dry-run 核对数据库状态")
		return 1
	}
	return 0
}
