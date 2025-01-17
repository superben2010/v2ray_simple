package main

import (
	"go.uber.org/zap"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer/tproxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func listenTproxy(addr string) {
	utils.Info("Start running Tproxy")

	ad, err := netLayer.NewAddr(addr)
	if err != nil {
		panic(err)
	}
	//tproxy因为比较特殊, 不属于 proxy.Server, 需要独特的转发过程去处理.
	lis, err := startLoopTCP(ad)
	if err != nil {
		if ce := utils.CanLogErr("TProxy startLoopTCP failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}
	udpConn := startLoopUDP(ad)

	tproxyList = append(tproxyList, tproxy.Machine{Addr: ad, Listener: lis, UDPConn: udpConn})

}

//非阻塞
func startLoopTCP(ad netLayer.Addr) (net.Listener, error) {
	return netLayer.ListenAndAccept("tcp", ad.String(), &netLayer.Sockopt{TProxy: true}, func(conn net.Conn) {
		tcpconn := conn.(*net.TCPConn)
		targetAddr := tproxy.HandshakeTCP(tcpconn)

		if ce := utils.CanLogInfo("TProxy loop read got new tcp"); ce != nil {
			ce.Write(zap.String("->", targetAddr.String()))
		}

		passToOutClient(incomingInserverConnState{
			wrappedConn:   tcpconn,
			defaultClient: defaultOutClient,
		}, false, tcpconn, nil, targetAddr)
	})

}

//非阻塞
func startLoopUDP(ad netLayer.Addr) *net.UDPConn {
	ad.Network = "udp"
	conn, err := ad.ListenUDP_withOpt(&netLayer.Sockopt{TProxy: true})
	if err != nil {
		if ce := utils.CanLogErr("TProxy startLoopUDP DialWithOpt failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}
	udpConn := conn.(*net.UDPConn)
	go func() {

		for {
			msgConn, raddr, err := tproxy.HandshakeUDP(udpConn)
			if err != nil {
				if ce := utils.CanLogErr("TProxy startLoopUDP loop read failed"); ce != nil {
					ce.Write(zap.Error(err))
				}
				break
			} else {
				if ce := utils.CanLogInfo("TProxy loop read got new udp"); ce != nil {
					ce.Write(zap.String("->", raddr.String()))
				}
			}

			go passToOutClient(incomingInserverConnState{
				defaultClient: defaultOutClient,
			}, false, nil, msgConn, raddr)
		}

	}()
	return udpConn
}
