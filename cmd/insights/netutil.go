package main

import (
	"net"
	"strings"
)

// containerInterfacePrefixes 列出纯容器/虚拟桥接网卡名前缀。
// 这些网卡只服务于本机容器内网，外部设备不可达，不作为可访问地址展示。
//
// 注意：不按接口「类型」(tap/tun) 排除——Tailscale(tun)、ZeroTier(tap) 等 overlay
// 网络同样是 tun/tap 设备，按类型排除会误伤它们；这里只按网卡名前缀判断。
var containerInterfacePrefixes = []string{
	"docker", // docker0、docker_gwbridge
	"br-",    // docker-compose / 自定义 bridge
	"veth",   // 容器虚拟以太网对
	"virbr",  // libvirt 默认网桥
	"bridge", // 通用 bridge
	"lxcbr",  // LXC 网桥
}

// isContainerInterface 判断网卡名是否属于容器/虚拟桥接。
func isContainerInterface(name string) bool {
	for _, p := range containerInterfacePrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// lanURL 表示一条外部可访问的 dashboard 入口及其所属网卡。
type lanURL struct {
	Iface string
	URL   string
}

// filterAccessibleIPv4 是筛选核心（纯函数，便于单测）：
// 给定一张网卡的属性与地址字符串列表（形如 "1.2.3.4/24"），返回其中外部可达的 IPv4。
// 排除：非 UP、loopback、容器网卡、IPv6、链路本地(169.254/16) 与回环(127/8)。
func filterAccessibleIPv4(ifaceName string, ifaceUp, ifaceLoopback bool, addrs []string) []string {
	if !ifaceUp || ifaceLoopback || isContainerInterface(ifaceName) {
		return nil
	}
	var out []string
	for _, a := range addrs {
		host := a
		if i := strings.Index(a, "/"); i >= 0 {
			host = a[:i]
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.To4() == nil {
			continue // 仅取 IPv4，避免 IPv6 隐私地址噪音
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		out = append(out, ip.String())
	}
	return out
}

// portFromAddr 从监听地址（":8932" / "0.0.0.0:8932"）提取端口；无法解析返回空串。
func portFromAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return port
}

// accessibleDashboardURLs 枚举本机网卡，返回所有外部可达的 dashboard URL。
// 启动时用于向局域网内、Tailscale/ZeroTier 等 overlay 内的设备展示可访问入口。
func accessibleDashboardURLs(listenAddr string) []lanURL {
	port := portFromAddr(listenAddr)
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []lanURL
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		addrStrs := make([]string, 0, len(addrs))
		for _, a := range addrs {
			addrStrs = append(addrStrs, a.String())
		}
		for _, ip := range filterAccessibleIPv4(iface.Name, iface.Flags&net.FlagUp != 0, iface.Flags&net.FlagLoopback != 0, addrStrs) {
			host := ip
			if port != "" {
				host = net.JoinHostPort(ip, port)
			}
			out = append(out, lanURL{Iface: iface.Name, URL: "http://" + host + "/dashboard"})
		}
	}
	return out
}
