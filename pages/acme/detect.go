//go:build windows

package acme

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	ipv4Services = []string{
		"https://api.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://checkip.amazonaws.com",
		"https://api4.my-ip.io/ip",
	}
	ipv6Services = []string{
		"https://api6.ipify.org",
		"https://ipv6.icanhazip.com",
		"https://api6.my-ip.io/ip",
	}
)

// detectPublicIP queries multiple services to detect the machine's public IP.
// If ipv6 is true it queries IPv6-only services, otherwise IPv4-only.
func detectPublicIP(ctx context.Context, ipv6 bool) (string, error) {
	services := ipv4Services
	if ipv6 {
		services = ipv6Services
	}

	// Shuffle to spread load and avoid always hitting the same service.
	order := rand.Perm(len(services))

	for _, i := range order {
		ip, err := queryIPService(ctx, services[i])
		if err == nil {
			return ip, nil
		}
	}
	if ipv6 {
		return "", fmt.Errorf("could not detect public IPv6 address")
	}
	return "", fmt.Errorf("could not detect public IPv4 address")
}

func queryIPService(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP %q from %s", ip, url)
	}
	return ip, nil
}
