package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	command := os.Args[1]

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		fatal("connect database: %v", err)
	}
	defer pool.Close()

	switch command {
	case "create":
		create(ctx, pool, os.Args[2:])
	case "passwd":
		passwd(ctx, pool, os.Args[2:])
	case "list":
		list(ctx, pool)
	default:
		usage()
		os.Exit(1)
	}
}

func create(ctx context.Context, pool *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	username := fs.String("username", "", "登录名")
	password := fs.String("password", "", "密码（至少 8 位）")
	role := fs.String("role", "super_admin", "角色：super_admin/editor/reviewer/ops")
	displayName := fs.String("name", "", "显示名")
	_ = fs.Parse(args)

	if *username == "" || len(*password) < 8 {
		fatal("username 必填，password 至少 8 位")
	}

	var roleID string
	if err := pool.QueryRow(ctx, `select id from admin_role where code = $1`, *role).Scan(&roleID); err != nil {
		fatal("角色不存在：%s", *role)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fatal("hash password: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		fatal("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx, `
		insert into admin_user (username, password_hash, display_name)
		values ($1, $2, $3) returning id`,
		*username, string(hash), *displayName).Scan(&id); err != nil {
		fatal("create admin: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`insert into admin_user_role (user_id, role_id) values ($1, $2)`, id, roleID); err != nil {
		fatal("assign role: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		fatal("commit: %v", err)
	}
	fmt.Printf("✔ 管理员已创建：%s（%s）\n", *username, *role)
}

func passwd(ctx context.Context, pool *pgxpool.Pool, args []string) {
	fs := flag.NewFlagSet("passwd", flag.ExitOnError)
	username := fs.String("username", "", "登录名")
	password := fs.String("password", "", "新密码（至少 12 位）")
	_ = fs.Parse(args)

	if *username == "" || utf8.RuneCountInString(*password) < 12 {
		fatal("username 必填，password 至少 12 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		fatal("hash password: %v", err)
	}
	var id string
	err = pool.QueryRow(ctx,
		`update admin_user set password_hash = $2 where username = $1 returning id`,
		*username, string(hash)).Scan(&id)
	if err != nil {
		fatal("update password: %v", err)
	}
	bumpTokenEpoch(ctx, id)
	fmt.Printf("✔ 密码已更新：%s（旧会话已失效）\n", *username)
}

// bumpTokenEpoch 递增令牌版本，使改密前签发的 JWT 立即失效。
// 没有 REDIS_URL 时只能依赖令牌自然过期，明确提示而不是静默跳过。
func bumpTokenEpoch(ctx context.Context, adminID string) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		fmt.Fprintln(os.Stderr, "提示：未设置 REDIS_URL，旧会话会保持有效直到 8 小时后过期")
		return
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "提示：REDIS_URL 不合法（%v），旧会话仍有效\n", err)
		return
	}
	client := redis.NewClient(opt)
	defer client.Close()
	if err := client.Incr(ctx, "admin:epoch:"+adminID).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "警告：令牌版本递增失败（%v），旧会话仍有效\n", err)
	}
}

func list(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx, `
		select u.username, coalesce(u.display_name, ''), u.status,
		       coalesce(string_agg(r.code, ',' order by r.code), '')
		from admin_user u
		left join admin_user_role ur on ur.user_id = u.id
		left join admin_role r on r.id = ur.role_id
		group by u.id
		order by u.username`)
	if err != nil {
		fatal("query admins: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var username, displayName, status, roles string
		if err := rows.Scan(&username, &displayName, &status, &roles); err != nil {
			fatal("scan: %v", err)
		}
		fmt.Printf("%-20s %-12s %-10s %s\n", username, displayName, status, roles)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, strings.TrimSpace(`
用法：
  go run ./cmd/admin create -username admin -password '********' -role super_admin -name 管理员
  go run ./cmd/admin passwd -username admin -password '********'
  go run ./cmd/admin list
`))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
