package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

type TcpProxyConn struct {
	conn net.Conn
}

const socketV5 byte = 0x05
const noAuth byte = 0x00
const passwordAuth byte = 0x02
const authCheckPass byte = 0x00
const authCheckNotPass byte = 0x01

const commandConnect byte = 0x01
const commandBind byte = 0x02
const commandUdp byte = 0x03

const addressTypeIpV4 byte = 0x01
const addressTypeDomain byte = 0x03
const addressTypeIpV6 byte = 0x04

const commandResponseOk byte = 0x00
const commandResponseNotSupport byte = 0x07

func (tcpProxyConn TcpProxyConn) handTcpProxy() {
	defer tcpProxyConn.conn.Close()

	_, _, _, err := tcpProxyConn.readProxyRequest()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	err = tcpProxyConn.chooseProxyMethod()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	//userName, password, err := tcpProxyConn.readPassword()
	//if err != nil {
	//	fmt.Printf("Error: %v\n", err)
	//	return
	//}
	//err = tcpProxyConn.authCheck(userName, password)
	//if err != nil {
	//	fmt.Printf("Error: %v\n", err)
	//	return
	//}

	address, port, addressType, err := tcpProxyConn.receiveCommand()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	remoteServerIp, err := resolveRemoteServerAddress(address, port, addressType[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Proxy Server Request: %v\n", remoteServerIp.String())
	remoteServer, err := net.Dial("tcp", remoteServerIp.String())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	err = tcpProxyConn.acceptCommand(remoteServer.LocalAddr().(*net.TCPAddr))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	wait := sync.WaitGroup{}
	wait.Add(1)
	go func() {
		io.Copy(remoteServer, tcpProxyConn.conn)
		defer wait.Done()
	}()
	wait.Add(1)
	go func() {
		io.Copy(tcpProxyConn.conn, remoteServer)
		defer wait.Done()
	}()
	defer remoteServer.Close()
	wait.Wait()
}

/**
 * return socks version, method count and methods.
 */
func (tcpProxyConn TcpProxyConn) readProxyRequest() (byte, byte, []byte, error) {
	client := tcpProxyConn.conn
	version := make([]byte, 1)
	methodCount := make([]byte, 1)
	_, err := client.Read(version)
	if err != nil {
		return version[0], methodCount[0], nil, err
	}
	_, err = client.Read(methodCount)
	if err != nil {
		return version[0], methodCount[0], nil, err
	}

	methods := make([]byte, int(methodCount[0]))
	_, err = client.Read(methods)
	if err != nil {
		return version[0], methodCount[0], nil, err
	}
	return version[0], methodCount[0], methods, nil
}

func (tcpProxyConn TcpProxyConn) chooseProxyMethod() error {
	client := tcpProxyConn.conn
	_, err := client.Write([]byte{socketV5, noAuth})
	return err
}

/**
 * return user name and password.
 */
func (tcpProxyConn TcpProxyConn) readPassword() ([]byte, []byte, error) {
	client := tcpProxyConn.conn
	version := make([]byte, 1)
	dataLength := make([]byte, 1)
	_, err := client.Read(version)
	if err != nil {
		return nil, nil, err
	}
	_, err = client.Read(dataLength)
	if err != nil {
		return nil, nil, err
	}
	userName := make([]byte, int(dataLength[0]))
	_, err = client.Read(userName)
	if err != nil {
		return nil, nil, err
	}
	_, err = client.Read(dataLength)
	if err != nil {
		return nil, nil, err
	}
	password := make([]byte, int(dataLength[0]))
	_, err = client.Read(password)
	if err != nil {
		return nil, nil, err
	}
	return userName, password, nil
}

// TODO: Auth Check.
func (tcpProxyConn TcpProxyConn) authCheck(userName []byte, password []byte) error {
	client := tcpProxyConn.conn
	_, err := client.Write([]byte{socketV5, authCheckPass})
	return err
}

type CommandNotSupport struct{}

func (e *CommandNotSupport) Error() string {
	return "Socks command not support"
}

/**
 * Return address, port and address type.
 */
func (tcpProxyConn TcpProxyConn) receiveCommand() ([]byte, []byte, []byte, error) {

	client := tcpProxyConn.conn

	version := make([]byte, 1)
	command := make([]byte, 1)
	rsv := make([]byte, 1)
	addrType := make([]byte, 1)
	_, err := client.Read(version)
	if err != nil {
		return nil, nil, nil, err
	}
	_, err = client.Read(command)
	if err != nil {
		return nil, nil, nil, err
	}
	if command[0] != commandConnect {
		return nil, nil, nil, &CommandNotSupport{}
	}
	_, err = client.Read(rsv)
	if err != nil {
		return nil, nil, nil, err
	}
	_, err = client.Read(addrType)
	if err != nil {
		return nil, nil, nil, err
	}

	var addressLen int
	switch addrType[0] {
	case addressTypeIpV4:
		addressLen = 4
	case addressTypeDomain:
		addrLen := make([]byte, 1)
		_, err = client.Read(addrLen)
		if err != nil {
			return nil, nil, nil, err
		}
		addressLen = int(addrLen[0])
	case addressTypeIpV6:
		addressLen = 16
	default:
		return nil, nil, nil, &CommandNotSupport{}
	}
	address := make([]byte, addressLen)
	_, err = client.Read(address)
	if err != nil {
		return nil, nil, nil, err
	}
	port := make([]byte, 2)
	_, err = client.Read(port)
	if err != nil {
		return nil, nil, nil, err
	}
	return address, port, addrType, nil
}

func resolveRemoteServerAddress(address, port []byte, addressType byte) (*net.TCPAddr, error) {
	portInt := int(binary.BigEndian.Uint16(port))
	switch addressType {
	case addressTypeIpV4:
		ip := net.IPv4(address[0], address[1], address[2], address[3])
		fmt.Printf("Request IPv4:%v, port: %v\n", ip.String(), portInt)
		return &net.TCPAddr{IP: ip, Port: portInt}, nil
	case addressTypeDomain:
		domain := string(address)
		fmt.Printf("Request Domain: %v, port: %v \n", domain, portInt)
		ips, err := net.LookupIP(domain)
		if err != nil {
			return nil, err
		}
		var ipv4 net.IP
		for _, ip := range ips {
			if ip.To4() != nil {
				ipv4 = ip
				break
			}
		}
		if ipv4 == nil {
			return nil, &CommandNotSupport{}
		}
		return &net.TCPAddr{IP: ipv4, Port: portInt}, nil
	default:
		return nil, &CommandNotSupport{}
	}
}

func (tcpProxyConn TcpProxyConn) acceptCommand(localAddr *net.TCPAddr) error {
	client := tcpProxyConn.conn
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(localAddr.Port))
	_, err := client.Write([]byte{socketV5, commandResponseOk, 0x00, addressTypeIpV4, localAddr.IP[0], localAddr.IP[1], localAddr.IP[2], localAddr.IP[3], portBytes[0], portBytes[1]})
	return err
}
