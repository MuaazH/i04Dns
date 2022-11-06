package util

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func TimeNowSeconds() int64 {
	return time.Now().Unix()
}

func TimeNowMilliseconds() int64 {
	return time.Now().UnixMilli()
}

func TimeNowString() string {
	t := time.Now()
	str := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return str
}

func IsValidIP(ip *string) bool {
	return ip != nil && net.ParseIP(*ip) != nil
}

func IsIp4(ip net.IP) bool {
	return ip.To4() != nil
}

func IsValidIP4(ip *string) bool {
	if ip == nil {
		return false
	}
	x := net.ParseIP(*ip)
	if x == nil {
		return false
	}
	return x.To4() != nil
}

func IsValidIP6(ip *string) bool {
	if ip == nil {
		return false
	}
	x := net.ParseIP(*ip)
	if x == nil {
		return false
	}
	return x.To16() != nil
}

func Ip4Bytes(ip string) *[4]byte {
	x := net.ParseIP(ip)
	if x == nil {
		return nil
	}
	y := net.IP.To4(x)
	return &([4]byte{y[0], y[1], y[2], y[3]})
}

func Ip6Bytes(ip string) *[16]byte {
	y := net.ParseIP(ip)
	if y == nil {
		return nil
	}
	x := net.IP.To16(y)
	return &([16]byte{x[0x0], x[0x1], x[0x2], x[0x3], x[0x4], x[0x5], x[0x6], x[0x7], x[0x8], x[0x9], x[0xA], x[0xB], x[0xC], x[0xD], x[0xE], x[0xF]})
}

func IsValidTCPNetwork(net string) bool {
	return net == "tcp" || net == "tcp4" || net == "tcp6"
}

func IsValidUDPNetwork(net string) bool {
	return net == "udp" || net == "udp4" || net == "udp6"
}

func IsValidHostName(name *string) bool {
	// todo: add some regex shit
	return !IsNullOrBlank(name)
}

func IsNullOrBlank(name *string) bool {
	return name == nil || len(*name) == 0 || len(strings.Trim(*name, " \t\n\r")) == 0
}
