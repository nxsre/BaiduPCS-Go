package pcscommand

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/network"
)

func TestParseBaiduCookies(t *testing.T) {
	cookies := []*network.Cookie{
		{Name: "BDUSS", Value: "root-bduss", Domain: ".baidu.com"},
		{Name: "BDUSS", Value: "pan-bduss", Domain: "pan.baidu.com"},
		{Name: "STOKEN", Value: "root-stoken", Domain: ".baidu.com"},
		{Name: "STOKEN", Value: "pan-stoken", Domain: "pan.baidu.com"},
		{Name: "BAIDUID", Value: "uid123", Domain: ".baidu.com"},
		{Name: "ignored", Value: "x", Domain: "example.com"},
	}

	bduss, stoken, cookieStr := parseBaiduCookies(cookies)
	if bduss != "pan-bduss" {
		t.Fatalf("bduss = %q, want pan-bduss", bduss)
	}
	if stoken != "pan-stoken" {
		t.Fatalf("stoken = %q, want pan-stoken", stoken)
	}
	if cookieStr == "" {
		t.Fatal("cookieStr is empty")
	}
	if !strings.Contains(cookieStr, "BDUSS=pan-bduss") || !strings.Contains(cookieStr, "STOKEN=pan-stoken") || !strings.Contains(cookieStr, "BAIDUID=uid123") {
		t.Fatalf("unexpected cookieStr: %s", cookieStr)
	}
}
