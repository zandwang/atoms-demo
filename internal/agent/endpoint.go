package agent

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
)

const maxBaseURLLength = 2048

// ValidateBaseURL checks the URL syntax without making a network request.
// DNS and address policy are checked again immediately before the request.
func ValidateBaseURL(raw string) (string, error) {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	if value == "" || len(value) > maxBaseURLLength {
		return "", endpointError("模型 endpoint 无效，请填写完整的 http(s) 地址。")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", endpointError("模型 endpoint 无效，请填写完整的 http(s) 地址。")
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return "", endpointError("模型 endpoint 无效，请填写完整的 http(s) 地址。")
	}
	return value, nil
}

func validateEndpointNetwork(ctx context.Context, baseURL string, allowPrivate bool) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return endpointError("模型 endpoint 无效，请检查地址后重试。")
	}
	host := parsed.Hostname()
	if allowPrivate {
		return nil
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return endpointError("当前服务禁止访问本机 endpoint。")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return &Error{Code: ErrorUpstreamUnavailable, Message: "模型 endpoint 无法解析，请检查地址后重试。", Retryable: true, Cause: err}
	}
	if len(ips) == 0 {
		return endpointError("模型 endpoint 没有可用地址。")
	}
	for _, ip := range ips {
		if isPrivateOrReservedIP(ip) {
			return endpointError("当前服务禁止访问私网模型 endpoint。")
		}
	}
	return nil
}

func isPrivateOrReservedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || ipInCIDRs(ip, []string{
		"100.64.0.0/10", // shared address space
		"192.0.0.0/24",  // IETF protocol assignments
		"198.18.0.0/15", // benchmark networks
		"240.0.0.0/4",   // reserved IPv4
	})
}

func ipInCIDRs(ip net.IP, cidrs []string) bool {
	for _, value := range cidrs {
		_, network, err := net.ParseCIDR(value)
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func endpointError(message string) error {
	return &Error{Code: ErrorEndpointInvalid, Message: message, Retryable: false, Cause: errors.New(message)}
}

func safeDialContext(ctx context.Context, network, address string, allowPrivate bool) (net.Conn, error) {
	if !allowPrivate {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, endpointError("模型 endpoint 地址无效，请检查地址后重试。")
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, &Error{Code: ErrorUpstreamUnavailable, Message: "模型 endpoint 无法解析，请检查地址后重试。", Retryable: true, Cause: err}
		}
		for _, ip := range ips {
			if isPrivateOrReservedIP(ip) {
				return nil, endpointError("当前服务禁止访问私网模型 endpoint。")
			}
		}
	}
	return (&net.Dialer{}).DialContext(ctx, network, address)
}
