package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestIsContainerInterface(t *testing.T) {
	cases := map[string]bool{
		"docker0":         true,
		"docker_gwbridge": true,
		"br-9c665a808e0a": true,
		"veth2eac7c5@if2": true,
		"virbr0":          true,
		"bridge0":         true,
		"lxcbr0":          true,
		"enp4s0":          false,
		"eth0":            false,
		"wls6":            false,
		"wlan0":           false,
		"tailscale0":      false,
		"ztrfymezre":      false, // ZeroTier：随机名，非容器
	}
	for name, want := range cases {
		if got := isContainerInterface(name); got != want {
			t.Errorf("isContainerInterface(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestFilterAccessibleIPv4(t *testing.T) {
	tests := []struct {
		name     string
		iface    string
		up       bool
		loopback bool
		addrs    []string
		want     []string
	}{
		{"有线局域网", "enp4s0", true, false, []string{"10.10.11.153/23", "fe80::df6c/64"}, []string{"10.10.11.153"}},
		{"Wi-Fi", "wls6", true, false, []string{"10.10.10.161/23", "2400:dd07::1/64"}, []string{"10.10.10.161"}},
		{"Tailscale overlay", "tailscale0", true, false, []string{"100.79.129.46/32"}, []string{"100.79.129.46"}},
		{"ZeroTier overlay", "ztrfymezre", true, false, []string{"172.28.113.11/16"}, []string{"172.28.113.11"}},
		{"Docker 网桥排除", "docker0", true, false, []string{"172.17.0.1/16"}, nil},
		{"自定义 bridge 排除", "br-9c665a808e0a", true, false, []string{"172.18.0.1/16"}, nil},
		{"veth 排除", "veth2eac7c5@if2", true, false, []string{"fe80::1/64"}, nil},
		{"loopback 排除", "lo", true, true, []string{"127.0.0.1/8"}, nil},
		{"接口 down 排除", "enp5s0", false, false, []string{"192.168.1.1/24"}, nil},
		{"链路本地排除", "enp4s0", true, false, []string{"169.254.1.1/16", "10.0.0.1/24"}, []string{"10.0.0.1"}},
		{"仅 IPv6 不取", "wls6", true, false, []string{"fe80::1/64", "2400:dd07::1/64"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterAccessibleIPv4(tt.iface, tt.up, tt.loopback, tt.addrs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterAccessibleIPv4(%q, %v, %v, %v) = %v, want %v",
					tt.iface, tt.up, tt.loopback, tt.addrs, got, tt.want)
			}
		})
	}
}

func TestPortFromAddr(t *testing.T) {
	cases := map[string]string{
		":8932":          "8932",
		"0.0.0.0:8932":   "8932",
		"127.0.0.1:8080": "8080",
		"8932":           "", // 缺冒号，无法解析
	}
	for addr, want := range cases {
		if got := portFromAddr(addr); got != want {
			t.Errorf("portFromAddr(%q) = %q, want %q", addr, got, want)
		}
	}
}

// TestAccessibleDashboardURLs 不依赖具体 IP（运行环境不定），仅校验端口拼接与 scheme 正确。
func TestAccessibleDashboardURLs(t *testing.T) {
	for _, u := range accessibleDashboardURLs(":8932") {
		if !strings.HasPrefix(u.URL, "http://") {
			t.Errorf("URL 缺少 scheme: %s", u.URL)
		}
		if !strings.HasSuffix(u.URL, ":8932/dashboard") {
			t.Errorf("URL 未带正确端口: %s", u.URL)
		}
		if u.Iface == "" {
			t.Errorf("URL 缺少网卡名: %+v", u)
		}
	}
}
