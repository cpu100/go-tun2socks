package ipstack

/*
#include "c/include/lwip/udp.h"
#include "c/include/lwip/tcp.h"
*/
import "C"
import (
    "fmt"
    "log"
    "unsafe"
)

//export udpRecvFn
func udpRecvFn(arg unsafe.Pointer, pcb *C.struct_udp_pcb, p *C.struct_pbuf, addr *C.ip_addr_t, port C.u16_t, destAddr *C.ip_addr_t, destPort C.u16_t) {
    if p == nil {
        log.Println("udpRecvFn", "buf == nil")
        return
    }
    var is = (*ipStack)(arg)
    conn := newUdpConn(pcb, addr, port, destAddr, destPort, is)

    go func() {
        if nil != is.tunHandler.UdpHandle(conn) {
            is.Lock()
            defer is.Unlock()
            C.pbuf_free(p)
            return
        }
        conn.chRead <- p
    }()
}

//export tcpAcceptFn
func tcpAcceptFn(arg unsafe.Pointer, pcb *C.struct_tcp_pcb, err C.err_t) C.err_t {
    go func() {
        var is = (*ipStack)(arg)

        err := is.tunHandler.TcpHandle(newTcpConn(pcb, is))

        is.Lock()
        defer is.Unlock()

        if nil != err {
            C.tcp_abort(pcb)
        } else if nil != pcb.refused_data {
            C.tcp_process_refused_data(pcb)
        }
    }()
    return C.ERR_OK
}

//export tcpRecvFn
func tcpRecvFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb, p *C.struct_pbuf, err C.err_t) C.err_t {
    if err != C.ERR_OK {
        log.Println("tcpRecvFn", err)
        return err
    }

    var conn = (*TCPConn)(arg)

    if nil == conn.pcb {
        if nil != p {
            C.pbuf_free(p)
        }
        return C.ERR_OK
    }

    // EOF
    if nil == p {
        // close(conn.chRead)
        conn.chRead <- nil
        // 如果关闭之后对面还发 RST 过来 lwip 会闪退
        // C.tcp_close(tpcb)
        // 都不要 close 除了 Close 方法和析构函数
        // C.tcp_shutdown(tpcb, 0, 1)
        // tcpPollFn 停止意味着 lwip 将其释放了
        // todo 在源码中找到资源变量，确保所有内存在控制中
        return C.ERR_OK
    }

    if len(conn.chRead) < 8 {
        fmt.Println("tcpRecvFn", conn.remoteAddr, unsafe.Pointer(p))
        conn.chRead <- p
    } else {
        log.Println("tcpRecvFn", "ERR_CONN")
        // Tell lwip we can't receive data at the moment, lwip will store it and try again later.
        return C.ERR_CONN
    }

    // for len(conn.chRead) > 0 {
    //     log.Println("debug: len(conn.chRead) > 0")
    //     time.Sleep(time.Millisecond * 50)
    // }

    return C.ERR_OK
}

//export tcpSentFn
func tcpSentFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb, len C.u16_t) C.err_t {
    var conn = (*TCPConn)(arg)
    conn.condSent.Signal()
    return C.ERR_OK
}

//export tcpErrFn
func tcpErrFn(arg unsafe.Pointer, errno C.err_t) {
    var conn = (*TCPConn)(arg)
    if nil != conn.pcb {
        conn.pcb = nil
        conn.chRead <- nil
    }
    switch errno {
    case C.ERR_RST:
        fmt.Println("ERR_RST: the connection was reset by the remote host", conn.localAddr, conn.remoteAddr)
    case C.ERR_ABRT:
        fmt.Println("ERR_ABRT: aborted through tcp_abort or by a TCP timer", conn.localAddr, conn.remoteAddr)
    default:
        fmt.Println("tcpErrFn", errno, conn.localAddr, conn.remoteAddr)
    }
}

//export tcpPollFn
func tcpPollFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb) C.err_t {
    var conn = (*TCPConn)(arg)
    fmt.Println("tcpPollFn", conn.remoteAddr.String())
    return C.ERR_OK
}

//export outputIp4
func outputIp4(netif *C.struct_netif, p *C.struct_pbuf, addr *C.ip4_addr_t) C.err_t {
    totlen := int(p.tot_len)
    var is = (*ipStack)(netif.state)
    if p.tot_len == p.len {
        buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:totlen:totlen]
        is.tunHandler.Write(buf[:totlen])
    } else {
        buf := make([]byte, totlen)
        C.pbuf_copy_partial(p, unsafe.Pointer(&buf[0]), p.tot_len, 0)
        is.tunHandler.Write(buf[:totlen])
    }
    return C.ERR_OK
}
