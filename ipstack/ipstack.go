package ipstack

/*
#cgo CFLAGS: -I./c/include
#include "lwip/tcp.h"
#include "lwip/udp.h"
#include "lwip/sys.h"
#include "lwip/init.h"
#include "c/custom/tool/tool.h"

extern err_t outputIp4(struct netif *netif,struct pbuf *buf, const ip4_addr_t *addr);

err_t outputIp6(struct netif *netif,struct pbuf *buf, const ip6_addr_t *addr) {
    return outputIp4(netif, buf, NULL);
}

err_t netif_init_cgo(struct netif *netif) {
    netif->mtu = 1500;
    netif->output = outputIp4;
    netif->output_ip6 = outputIp6;
    return ERR_OK;
}

struct netif* netif_add_cgo (uintptr_t state) {
    struct netif *netif = (struct netif*)mem_malloc(sizeof(struct netif));
    return netif_add_noaddr (netif, (void*)state, netif_init_cgo, ip_input);
}

extern err_t tcpAcceptFn(void *arg, struct tcp_pcb *newpcb, err_t err);
extern void udpRecvFn(void *arg, struct udp_pcb *pcb, struct pbuf *p, const ip_addr_t *addr, u16_t port, const ip_addr_t *dest_addr, u16_t dest_port);

err_t netif_input_cgo(struct pbuf *buf, struct netif *netif) {
    return (*netif).input(buf, netif);
}

// 修改 .c .h 文件后
;;;;;;;;;;;;;;;;;;;;;;;;;;;
// 在这里随便加几个分号即可清除 C 代码编译的缓存

*/
import "C"
import (
    "encoding/binary"
    "log"
    "sync"
    "unsafe"
)

func SetTunHandler(th TunHandler, thread int) {
    is := newIpStack(th)
    is.init()
    go func() {
        buffer := make([]byte, 1500)
        for {
            nr, err := th.Read(buffer)
            if nil != err {
                panic(err)
            }

            protocol := buffer[9]
            if 1 != protocol && 6 != protocol && 17 != protocol {
                log.Fatalln("Protocol", protocol)
            }
            // ip := buffer[16:20]
            port := binary.BigEndian.Uint16(buffer[22:24])
            if port == 0 {
                log.Fatalln("Port", port)
            }

            is.Lock()
            var buf *C.struct_pbuf
            buf = C.pbuf_alloc(C.PBUF_RAW, C.u16_t(nr), C.PBUF_POOL)
            C.pbuf_take(buf, unsafe.Pointer(&buffer[0]), C.u16_t(nr))
            ierr := C.netif_input_cgo(buf, is.netif)
            if ierr != C.ERR_OK {
                C.pbuf_free(buf)
                panic(ierr)
            }
            is.Unlock()
        }
    }()
    // thread = 1
    // var iss []*ipStack
    // var netifs []*C.struct_netif
    // var chBuffers []chan[]byte
    // for i := 0; i < thread; i++ {
    //     is := newIpStack(th)
    //     is.init()
    //     iss = append(iss, is)
    //     netifs = append(netifs, is.netif)
    //     chBuffers = append(chBuffers, make(chan []byte))
    //     go func(i int) {
    //         netif := netifs[i]
    //         chBuffer := chBuffers[i]
    //         for {
    //             buffer := <-chBuffer
    //             nr := len(buffer)
    //             var buf *C.struct_pbuf
    //             buf = C.pbuf_alloc(C.PBUF_RAW, C.u16_t(nr), C.PBUF_POOL)
    //             C.pbuf_take(buf, unsafe.Pointer(&buffer[0]), C.u16_t(nr))
    //             ierr := C.netif_input_cgo(buf, netif)
    //             if ierr != C.ERR_OK {
    //                 C.pbuf_free(buf)
    //                 panic(ierr)
    //             }
    //         }
    //     }(i)
    // }
    // go func() {
    //     buffer2 := make([]byte, 1500)
    //     for {
    //         _, err := th.Read(buffer2)
    //         if nil != err {
    //             panic(err)
    //         }
    //         // todo if ipv6
    //         i := binary.LittleEndian.Uint32(buffer2[16:20])%uint32(len(netifs))
    //         buffer := append(make([]byte, 0, len(buffer2)), buffer2...)
    //         chBuffers[i] <- buffer
    //         // fmt.Println(buffer[16:20])
    //         // fmt.Println("netif", i, len(netifs))
    //
    //     }
    // }()
    return
}

type ipStack struct {
    sync.Mutex

    netif *C.struct_netif

    tunHandler TunHandler
}

func newIpStack(th TunHandler) *ipStack {
    is := &ipStack{tunHandler: th}
    is.netif = C.netif_add_cgo(C.uintptr_t(uintptr(unsafe.Pointer(is))))
    return is
}

func (is *ipStack) init() {

    // TCP
    tcpPCB := C.tcp_new()
    if tcpPCB == nil {
        panic("tcp_new return nil")
    }

    C.tcp_bind_netif(tcpPCB, is.netif)
    errno := C.tcp_bind(tcpPCB, C.IP_ADDR_ANY, 0)
    if errno != C.ERR_OK {
        panic("tcp_bind error")
    }

    tcpPCB = C.tcp_listen_with_backlog(tcpPCB, C.TCP_DEFAULT_LISTEN_BACKLOG)
    if tcpPCB == nil {
        panic("can not allocate tcp pcb")
    }

    C.tcp_arg_cgo(tcpPCB, C.uintptr_t(uintptr(unsafe.Pointer(is))))
    C.tcp_accept(tcpPCB, Cptr(C.tcpAcceptFn))

    // UDP
    udpPCB := C.udp_new()
    if udpPCB == nil {
        panic("udp_new return nil")
    }

    C.udp_bind_netif(udpPCB, is.netif)
    errno = C.udp_bind(udpPCB, C.IP_ADDR_ANY, 0)
    if errno != C.ERR_OK {
        panic("udp_bind error")
    }

    C.udp_recv_cgo(udpPCB, Cptr(C.udpRecvFn), C.uintptr_t(uintptr(unsafe.Pointer(is))))
}

func init() {
    C.lwip_init()
    // go func() {
    //     ticker := time.NewTicker(time.Millisecond * 200)
    //     for {
    //         <-ticker.C
    //         C.sys_check_timeouts()
    //     }
    // }()
}
