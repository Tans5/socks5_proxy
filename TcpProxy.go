package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"socks5_proxy/utils"
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

	address, port, _, err := tcpProxyConn.receiveCommand()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Proxy Address: %v, Port: %v\n", string(address), int(binary.LittleEndian.Uint16(port)))
	err = tcpProxyConn.acceptCommand()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	remoteServer, err := net.Dial("tcp", net.IP{address[0], address[1], address[2], address[3]}.String() + ":" + string(binary.LittleEndian.Uint16(port)))
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
func (proxy TcpProxyConn) authCheck(userName []byte, password []byte) error {
	client := proxy.conn
	_, err := client.Write([]byte{socketV5, authCheckPass})
	return err
}

type CommandNotSupport struct {}

func (e *CommandNotSupport) Error() string {
	return "Socks command not support"
}

/**
 * Return address and port.
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
	addrLen := make([]byte, 1)
	_, err = client.Read(addrLen)
	if err != nil {
		return nil, nil, nil, err
	}
	address := make([]byte, int(addrLen[0]))
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

func (tcpProxyConn TcpProxyConn) acceptCommand() error {
	client := tcpProxyConn.conn
	ip, _ := utils.FindLocalIPV4()
	port := uint16(8081)
	portBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(portBytes, port)
	_, err := client.Write([]byte{socketV5, commandResponseOk, 0x00, addressTypeIpV4, 0x04, ip[0], ip[1], ip[2], ip[3], portBytes[0], portBytes[1]})
	return err
}

