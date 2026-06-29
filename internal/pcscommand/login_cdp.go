package pcscommand

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const defaultPanURL = "https://pan.baidu.com"

// CDPLoginOptions CDP 浏览器登录选项
type CDPLoginOptions struct {
	RemoteURL  string // 远程调试地址，如 http://127.0.0.1:9222
	ExecPath   string // Chrome/Chromium 可执行文件路径
	UserDataDir string // 浏览器用户数据目录，可复用已登录会话
	TimeoutSec int    // 等待登录超时（秒）
	TargetURL  string // 打开的登录页
}

// RunLoginCDP 通过 CDP 打开浏览器，等待用户在百度网盘完成登录后自动读取 Cookie
func RunLoginCDP(opts CDPLoginOptions) (bduss, ptoken, stoken, cookies string, err error) {
	if opts.TimeoutSec <= 0 {
		opts.TimeoutSec = 300
	}
	if opts.TargetURL == "" {
		opts.TargetURL = defaultPanURL
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.TimeoutSec)*time.Second)
	defer cancel()

	var allocCtx context.Context
	var allocCancel context.CancelFunc

	if opts.RemoteURL != "" {
		allocCtx, allocCancel = chromedp.NewRemoteAllocator(ctx, opts.RemoteURL)
	} else {
		allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", false),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("no-first-run", true),
			chromedp.Flag("no-default-browser-check", true),
		)
		if opts.ExecPath != "" {
			allocOpts = append(allocOpts, chromedp.ExecPath(opts.ExecPath))
		}
		if opts.UserDataDir != "" {
			allocOpts = append(allocOpts, chromedp.UserDataDir(opts.UserDataDir))
		}
		allocCtx, allocCancel = chromedp.NewExecAllocator(ctx, allocOpts...)
	}
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	fmt.Println("正在通过 CDP 打开浏览器，请在页面中完成百度网盘登录…")
	fmt.Printf("目标地址: %s\n", opts.TargetURL)
	if opts.RemoteURL != "" {
		fmt.Printf("已连接远程浏览器: %s\n", opts.RemoteURL)
	}
	fmt.Printf("等待登录完成（超时 %ds）…\n", opts.TimeoutSec)

	if err = chromedp.Run(browserCtx, network.Enable(), chromedp.Navigate(opts.TargetURL)); err != nil {
		return "", "", "", "", fmt.Errorf("打开浏览器失败: %w", err)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", "", "", fmt.Errorf("等待登录超时 (%ds)，未获取到 BDUSS", opts.TimeoutSec)
		case <-ticker.C:
			var networkCookies []*network.Cookie
			err = chromedp.Run(browserCtx, chromedp.ActionFunc(func(c context.Context) error {
				var getErr error
				networkCookies, getErr = network.GetCookies().Do(c)
				return getErr
			}))
			if err != nil {
				continue
			}

			bduss, stoken, cookies = parseBaiduCookies(networkCookies)
			if bduss == "" {
				continue
			}

			if stoken == "" {
				fmt.Println("警告: 未检测到 STOKEN，转存功能可能不可用；请确认已在 pan.baidu.com 完成登录")
			} else {
				fmt.Println("已检测到登录 Cookie（含 BDUSS / STOKEN）")
			}
			return bduss, "", stoken, cookies, nil
		}
	}
}

func parseBaiduCookies(cookies []*network.Cookie) (bduss, stoken, cookieStr string) {
	type cookieEntry struct {
		name   string
		value  string
		domain string
	}
	seen := make(map[string]cookieEntry)

	for _, c := range cookies {
		if !isBaiduDomain(c.Domain) {
			continue
		}
		key := c.Name + "|" + c.Domain
		seen[key] = cookieEntry{name: c.Name, value: c.Value, domain: c.Domain}
	}

	// BDUSS：优先 pan.baidu.com
	for _, e := range seen {
		if e.name == "BDUSS" && bduss == "" {
			bduss = e.value
		}
	}
	for _, e := range seen {
		if e.name == "BDUSS" && strings.Contains(e.domain, "pan.baidu") {
			bduss = e.value
			break
		}
	}

	// STOKEN 必须在网盘域获取
	for _, e := range seen {
		if e.name == "STOKEN" && strings.Contains(e.domain, "pan.baidu") {
			stoken = e.value
			break
		}
	}
	if stoken == "" {
		for _, e := range seen {
			if e.name == "STOKEN" {
				stoken = e.value
				break
			}
		}
	}

	// 去重后拼接 Cookie 字符串（同名取 pan.baidu 域）
	byName := make(map[string]cookieEntry)
	for _, e := range seen {
		prev, ok := byName[e.name]
		if !ok || cookieDomainPriority(e.domain) > cookieDomainPriority(prev.domain) {
			byName[e.name] = e
		}
	}
	parts := make([]string, 0, len(byName))
	for name, e := range byName {
		parts = append(parts, fmt.Sprintf("%s=%s", name, e.value))
	}
	cookieStr = strings.Join(parts, "; ")
	return bduss, stoken, cookieStr
}

func cookieDomainPriority(domain string) int {
	if strings.Contains(domain, "pan.baidu") {
		return 2
	}
	if strings.HasPrefix(domain, ".") || domain == "baidu.com" {
		return 1
	}
	return 0
}

func isBaiduDomain(domain string) bool {
	domain = strings.TrimPrefix(domain, ".")
	return domain == "baidu.com" || strings.HasSuffix(domain, ".baidu.com")
}
