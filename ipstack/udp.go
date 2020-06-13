package ipstack

/*
#include "c/include/lwip/udp.h"
#include "c/custom/tool/tool.h"
*/
import "C"
import (
    "errors"
    "fmt"
    "io"
    "net"
    "runtime"
    "time"
    "unsafe"
)

type UDPConn struct {
    localAddr  *net.UDPAddr
    remoteAddr *net.UDPAddr

    chRead chan *C.struct_pbuf

    rTimer    *time.Timer
    rDuration time.Duration

    is  *ipStack
    pcb *C.struct_udp_pcb
}

// func (conn *UDPConn) WriteTo(w io.Writer) (n int64, err error) {
//     panic("implement me")
// }
//
// func (conn *UDPConn) ReadFrom(r io.Reader) (n int64, err error) {
//     panic("implement me")
// }

func (conn *UDPConn) Read(b []byte) (n int, err error) {
    select {
    case <-conn.rTimer.C:
        poolPutTimer(conn.rTimer, false)
        return 0, ErrTimeout
    case p := <-conn.chRead:
        buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:p.len:p.len]
        if len(b) < len(buf) {
            return 0, io.ErrShortBuffer
        }
        copy(b, buf)
        C.pbuf_free(p)
        return len(buf), nil
    }
}

func (conn *UDPConn) Write(b []byte) (int, error) {
    conn.is.Lock()
    defer conn.is.Unlock()

    buf := C.pbuf_alloc_reference(unsafe.Pointer(&b[0]), C.u16_t(len(b)), C.PBUF_ROM)
    defer C.pbuf_free(buf)

    if len(conn.localAddr.IP) == 4 {
        C.ip_addr_type_set(&conn.pcb.local_ip, C.IPADDR_TYPE_V4)
        C.ip_addr_type_set(&conn.pcb.remote_ip, C.IPADDR_TYPE_V4)
        copy(conn.pcb.local_ip.u_addr[:4], conn.remoteAddr.IP)
        copy(conn.pcb.remote_ip.u_addr[:4], conn.localAddr.IP)
    } else {
        C.ip_addr_type_set(&conn.pcb.local_ip, C.IPADDR_TYPE_V6)
        C.ip_addr_type_set(&conn.pcb.remote_ip, C.IPADDR_TYPE_V6)
        copy(conn.pcb.local_ip.u_addr[4:], conn.remoteAddr.IP)
        copy(conn.pcb.remote_ip.u_addr[4:], conn.localAddr.IP)
    }

    err := C.udp_sendto_if_src(
        conn.pcb,
        buf,
        &conn.pcb.remote_ip,
        C.u16_t(conn.localAddr.Port),
        conn.is.netif,
        &conn.pcb.local_ip,
        C.u16_t(conn.remoteAddr.Port),
    )

    if err != C.ERR_OK {
        return 0, errors.New("udp send error")
    }
    return len(b), nil
}

func (conn *UDPConn) Close() error {
    conn.is.Lock()
    defer conn.is.Unlock()
    fmt.Println("close udp", conn.localAddr, conn.remoteAddr)

    if nil == conn.pcb {
        return ErrCloseClosed
    }

    // C.tcp_close(conn.pcb)
    // C.udp_remove(conn.pcb)
    // udp 的 pcb 是公共的，只有一个
    // 不像 tcp 那样有新的 pcb 持续保留到连接关闭
    conn.pcb = nil

    close(conn.chRead)
    for {
        p := <-conn.chRead
        if nil == p {
            break
        }
        C.pbuf_free(p)
    }

    return nil
}

func (conn *UDPConn) LocalAddr() net.Addr {
    return conn.localAddr
}

func (conn *UDPConn) RemoteAddr() net.Addr {
    return conn.remoteAddr
}

func (conn *UDPConn) SetDeadline(t time.Time) error {
    panic("implement me")
}

func (conn *UDPConn) SetReadDeadline(t time.Time) error {
    conn.rDuration = t.Sub(time.Now())
    if nil != conn.rTimer {
        poolPutTimer(conn.rTimer, true)
    }
    conn.rTimer = poolGetTimer(conn.rDuration)
    return nil
}

func (conn *UDPConn) SetWriteDeadline(t time.Time) error {
    panic("implement me")
}

func newUdpConn(pcb *C.struct_udp_pcb, addr *C.ip_addr_t, port C.u16_t, destAddr *C.ip_addr_t, destPort C.u16_t, is *ipStack) *UDPConn {
    conn := &UDPConn{
        is:     is,
        pcb:    pcb,
        chRead: make(chan *C.struct_pbuf, 8),
    }

    runtime.SetFinalizer(conn, (*UDPConn).Close)

    var remoteIp, localIp net.IP
    if C.IPADDR_TYPE_V4 == C.ip_addr_type(addr) {
        localIp = append(make([]byte, 0, 4), addr.u_addr[:4]...)
        remoteIp = append(make([]byte, 0, 4), destAddr.u_addr[:4]...)
    } else {
        localIp = append(make([]byte, 0, 16), addr.u_addr[4:]...)
        remoteIp = append(make([]byte, 0, 16), destAddr.u_addr[4:]...)
    }

    conn.localAddr = &net.UDPAddr{
        IP:   localIp,
        Port: int(port),
    }

    conn.remoteAddr = &net.UDPAddr{
        IP:   remoteIp,
        Port: int(destPort),
    }

    return conn
}
