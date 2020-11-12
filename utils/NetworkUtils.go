package utils

import (
	"net"
)

func FindLocalIPV4() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		var ip net.IP = nil
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
			return ip, nil
		}
	}
	return nil, nil
}

const PackageSize int = 1024 * 1.5

func ReadConnPackage(conn *net.UDPConn) ([]byte, *net.UDPAddr, error) {
	packageBytes := make([]byte, PackageSize)
	readCount, udpAddr, err := conn.ReadFromUDP(packageBytes)

	switch {
	case err != nil:
		return nil, nil, err
	case readCount <= 0:
		return nil, nil, nil
	default:
		return packageBytes[0:readCount], udpAddr, err
	}

}
