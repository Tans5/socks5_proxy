package main

import (
	"fmt"
	"net"
	"socks5_proxy/utils"
	"sync"
)

var waitJob = sync.WaitGroup{}

func main() {

	waitJob.Add(1)
	go proxyTcp()
	waitJob.Add(1)
	go proxyUdp()
	waitJob.Wait()

}

//func udpSender() {
//
//	localIp, err := utils.FindLocalIPV4()
//	if localIp == nil || err != nil {
//		return
//	}
//
//	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: localIp, Port: 2000})
//	if err != nil {
//		return
//	}
//	addr := conn.LocalAddr()
//	fmt.Printf("%v\n", addr)
//	conn.WriteToUDP(make([]byte, 1025), &net.UDPAddr{IP: localIp, Port: 1999})
//}

func proxyTcp() {
	localIp, err := utils.FindLocalIPV4()
	if err != nil {
		return
	}
	fmt.Printf("Local Addr: %v\n", localIp.String())
	server, err := net.Listen("tcp", localIp.String()+":8081")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for true {
		client, err := server.Accept()
		if err != nil || client == nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Tcp Remote Client Addr: %v\n", client.RemoteAddr())
		proxyConn := TcpProxyConn{conn: client}
		go proxyConn.handTcpProxy()
	}
	defer waitJob.Done()
	fmt.Printf("Tcp Proxy Exit\n")
}

func proxyUdp() {

	localIp, _ := utils.FindLocalIPV4()
	if localIp == nil {
		return
	}

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: localIp, Port: 8081})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	for true {
		data, addr, err := utils.ReadConnPackage(udpConn)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Udp Remote Data And Addr: %v, %d\n", addr, len(data))
	}

	defer waitJob.Done()
}
