package AddrLimiter

import (
	"errors"
	"net"
)

var whilteList = make(map[string]bool, 10)

func init() {
	whilteList["127.0.0.1"] = true
}

func get_ip(addr string) (string, error) {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	if net.ParseIP(ip) != nil {
		return ip, nil
	}
	return "", errors.New("ip parse invalid")
}

func IPEnable(ip string) (bool, error) {
	if enable, ok := whilteList[ip]; ok && enable {
		return true, nil
	}
	return false, errors.New("ip check failed")
}

func AddrEnabel(addr string) (bool, error) {
	if ip, err := get_ip(addr); err != nil {
		return false, err
	} else {
		return IPEnable(ip)
	}
}
