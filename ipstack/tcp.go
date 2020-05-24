package ipstack

/*
#include "c/include/lwip/tcp.h"
#include "c/custom/tool/tool.h"

extern err_t tcpRecvFn(void *arg, struct tcp_pcb *tpcb, struct pbuf *p, err_t err);
extern err_t tcpSentFn(void *arg, struct tcp_pcb *tpcb, u16_t len);
extern void tcpErrFn(void *arg, err_t err);
extern err_t tcpPollFn(void *arg, struct tcp_pcb *tpcb);

*/
import "C"
import (
    "errors"
    "fmt"
    "io"
    "net"
    "sync"
    "time"
    "unsafe"
)

type TCPConn struct {
    localAddr  *net.TCPAddr
    remoteAddr *net.TCPAddr

    chRead chan *C.struct_pbuf

    condSent *sync.Cond

    rTimer    *time.Timer
    rDuration time.Duration

    is  *ipStack
    pcb *C.struct_tcp_pcb
}

func (conn *TCPConn) WriteTo(w io.Writer) (n int64, err error) {
    var p *C.struct_pbuf
    for {
        select {
        case p = <-conn.chRead:
        case <-conn.rTimer.C:
            poolPutTimer(conn.rTimer, false)
            err = ErrTimeout
            return
        }
        if nil == p {
            return
        }
        for {
            buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:p.len:p.len]
            nw, err2 := w.Write(buf)
            n += int64(nw)
            if nil != err2 {
                // 暂不处理临时错误的情况
                err = errors.New("write to: " + err2.Error())
                return
            }
            if len(buf) != nw {
                err = io.ErrShortWrite
                return
            }
            if nil == p.next {
                C.pbuf_free(p)
                break
            } else {
                p = p.next
            }
        }
    }
}

func (conn *TCPConn) ReadFrom(r io.Reader) (n int64, err error) {
    b := make([]byte, 1500)
    for {
        nr, err2 := r.Read(b)
        if nil != err2 {
            if err2 != io.EOF {
                err = err2
            }
            break
        }
        errno := C.tcp_write(conn.pcb, unsafe.Pointer(&b[0]), C.u16_t(len(b)), C.TCP_WRITE_FLAG_COPY)
        if errno == C.ERR_OK {
            C.tcp_output(conn.pcb)
            n += int64(nr)
            continue
        } else {
            err = errors.New("write error")
            break
        }
    }
    return
}

func (conn *TCPConn) Read(b []byte) (int, error) {
    panic("implement me")
    // select {
    // case <-conn.rTimer.C:
    //     return 0, ErrTimeout
    // case bs := <-conn.chRead:
    //     if nil == bs {
    //         return 0, io.EOF
    //     }
    //     if len(b) < len(bs) {
    //         return 0, io.ErrShortBuffer
    //     }
    //     copy(b, bs)
    //     return len(bs), nil
    // }
}

func (conn *TCPConn) Write(b []byte) (int, error) {
    panic("implement me")
    // conn.is.Lock()
    // defer conn.is.Unlock()
    // err := C.tcp_write(conn.pcb, unsafe.Pointer(&b[0]), C.u16_t(len(b)), C.TCP_WRITE_FLAG_COPY)
    // if err == C.ERR_OK {
    //     C.tcp_output(conn.pcb)
    //     return len(b), nil
    // } else if err == C.ERR_MEM {
    //     return 0, errors.New("Write Error")
    // }
    // return 0, errors.New("Write Error")
}

func (conn *TCPConn) Close() error {
    fmt.Println("Close", conn.remoteAddr)
    C.tcp_close(conn.pcb)
    return nil
}

func (conn *TCPConn) LocalAddr() net.Addr {
    return conn.localAddr
}

func (conn *TCPConn) RemoteAddr() net.Addr {
    return conn.remoteAddr
}

func (conn *TCPConn) SetDeadline(t time.Time) error {
    panic("implement me")
}

func (conn *TCPConn) SetReadDeadline(t time.Time) error {
    conn.rDuration = t.Sub(time.Now())
    if nil != conn.rTimer {
        poolPutTimer(conn.rTimer, true)
    }
    conn.rTimer = poolGetTimer(conn.rDuration)
    return nil
}

func (conn *TCPConn) SetWriteDeadline(t time.Time) error {
    panic("implement me")
}

func newTcpConn(pcb *C.struct_tcp_pcb, is *ipStack) *TCPConn {
    conn := &TCPConn{
        is:       is,
        pcb:      pcb,
        chRead:   make(chan *C.struct_pbuf, 8),
        condSent: sync.NewCond(new(sync.Mutex)),
    }

    C.tcp_arg_cgo(pcb, C.uintptr_t(uintptr(unsafe.Pointer(conn))))
    C.tcp_recv(pcb, Cptr(C.tcpRecvFn))
    C.tcp_sent(pcb, Cptr(C.tcpSentFn))
    C.tcp_err(pcb, Cptr(C.tcpErrFn))
    C.tcp_poll(pcb, Cptr(C.tcpPollFn), C.u8_t(8))

    var remoteIp, localIp net.IP
    if C.IPADDR_TYPE_V4 == C.ip_addr_type(&pcb.local_ip) {
        // 在 go 中用不了 c union 成员名字，但能以 slice 形式访问
        // c union 在内存中存储的顺序和它们的名字顺序相反
        localIp = pcb.remote_ip.u_addr[:4]
        remoteIp = pcb.local_ip.u_addr[:4]
    } else {
        localIp = pcb.remote_ip.u_addr[4:]
        remoteIp = pcb.local_ip.u_addr[4:]
    }

    conn.localAddr = &net.TCPAddr{
        IP:   localIp,
        Port: int(pcb.remote_port),
    }

    conn.remoteAddr = &net.TCPAddr{
        IP:   remoteIp,
        Port: int(pcb.local_port),
    }

    return conn
}
