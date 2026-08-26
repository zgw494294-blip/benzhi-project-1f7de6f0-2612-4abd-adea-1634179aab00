package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	Address, DataPath string
	Selfcheck         bool
	Timeout           time.Duration
}

func resolveAddress(explicit string) (string, error) {
	address := strings.TrimSpace(explicit)
	if address == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return "", errors.New("PORT 必须是 1 到 65535 的端口号")
			}
			address = net.JoinHostPort("127.0.0.1", port)
		} else {
			address = defaultAddress
		}
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("监听地址无效: %w", err)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", errors.New("监听端口必须是 1 到 65535 的数字")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return "", errors.New("监听地址仅允许回环主机 127.0.0.1、::1 或 localhost")
	}
	return address, nil
}
