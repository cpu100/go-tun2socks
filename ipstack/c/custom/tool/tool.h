#include <stdint.h>
// #include "lwip/mem.h"
#include "lwip/tcp.h"
#include "lwip/udp.h"

// void* new_tcp_arg(uint32_t val);
// void free_tcp_arg(void *arg);

void tcp_arg_cgo(struct tcp_pcb *pcb, uintptr_t ptr);
void udp_recv_cgo(struct udp_pcb *pcb, udp_recv_fn recv, uintptr_t ptr);
u8_t ip_addr_type(struct ip_addr *addr);
void ip_addr_type_set(struct ip_addr *addr, enum lwip_ip_addr_type type);
