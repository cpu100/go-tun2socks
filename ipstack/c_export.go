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

    if pcb == nil {
        return
    }

    var is = (*ipStack)(arg)
    conn := newUdpConn(pcb, addr, port, destAddr, destPort, is)

    go func() {
        defer func() {
            if p != nil {
                C.pbuf_free(p)
            }
        }()

        is.tunHandler.UdpHandle(conn)

        for {
            fmt.Println("udpRecvFn")
            buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:p.len:p.len]
            conn.chRead <- buf
            if nil == p.next {
                // C.tcp_recved(conn.pcb, p.tot_len)
                break
            } else {
                p = p.next
            }
            conn.chRead <- buf
        }
    }()
}

//export tcpAcceptFn
func tcpAcceptFn(arg unsafe.Pointer, pcb *C.struct_tcp_pcb, err C.err_t) C.err_t {
    go func() {
        var is = (*ipStack)(arg)
        is.tunHandler.TcpHandle(newTcpConn(pcb, is))
        // if nil != err {
        //     C.tcp_abort(pcb)
        // }
        if pcb.refused_data != nil {
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

    // EOF
    if nil == p {
        close(conn.chRead)
        C.tcp_close(tpcb)
        return err
    }


    conn.chRead <- p
    C.tcp_recved(conn.pcb, p.tot_len)

    return C.ERR_OK
}

//export tcpSentFn
func tcpSentFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb, len C.u16_t) C.err_t {
    var conn = (*TCPConn)(arg)
    fmt.Println("tcpSentFn")
    conn.condSent.Signal()
    // C.tcp_close(conn.pcb)
    return C.ERR_OK
}

//export tcpErrFn
func tcpErrFn(arg unsafe.Pointer, errno C.err_t) {
    var conn = (*TCPConn)(arg)
    switch errno {
    case C.ERR_RST:
        fmt.Println("ERR_RST: the connection was reset by the remote host", conn.remoteAddr)
    case C.ERR_ABRT:
        fmt.Println("ERR_ABRT: aborted through tcp_abort or by a TCP timer", conn.remoteAddr)
    default:
        fmt.Println("tcpErrFn", errno, conn.remoteAddr)
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
