#include "tool/tool.h"

/*
void* new_tcp_arg(uint32_t val) {
    uint32_t *arg;
	arg = mem_malloc(sizeof(uint32_t));
	*arg = val;
	return arg;
}

void free_tcp_arg(void *arg) {
    mem_free(arg);
}
*/

// C.tcp_arg(tcpPCB, unsafe.Pointer(is))
// 改成
// C.tcp_arg_cgo(tcpPCB, C.ulonglong(uintptr(unsafe.Pointer(is))))
// 将 go 指针以数字形式传递
// 绕过 cgo 不允许向 c 函数传递 go 指针的错误 (cgo argument has Go pointer to Go pointer)
void tcp_arg_cgo(struct tcp_pcb *pcb, uintptr_t ptr) {
    tcp_arg(pcb, (void*)ptr);
}

void udp_recv_cgo(struct udp_pcb *pcb, udp_recv_fn recv, uintptr_t ptr) {
    udp_recv(pcb, recv, (void*)ptr);
}

/*
// fmt.Println((*[4 ]byte)(C.ip_addr_bytes(&pcb.local_ip)))
// fmt.Println((*[16]byte)(C.ip_addr_bytes(&pcb.local_ip)))
// 不方便在 go 中判断 ip 版本
void* ip_addr_bytes(struct ip_addr *addr) {
    if ( IPADDR_TYPE_V4 == addr->type ) {
        return &(*addr).u_addr.ip4;
    } else {
        return &(*addr).u_addr.ip6;
    }
}
*/

// 解决 type 在 go 中是关键字，无法访问的问题
u8_t ip_addr_type(struct ip_addr *addr) {
    return addr->type;
}

void ip_addr_type_set(struct ip_addr *addr, enum lwip_ip_addr_type type) {
    addr->type = type;
}
