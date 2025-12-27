// go:build ignore

// eBPF program for monitoring TCP traffic on specific ports.
// Uses kprobes on tcp_sendmsg and tcp_cleanup_rbuf for passive observation.

#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

char LICENSE[] SEC("license") = "GPL";

// ============================================================================
// Map Definitions
// ============================================================================

// Configuration map - holds target ports to monitor
struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 64);
  __type(key, __u16);  // port number
  __type(value, __u8); // enabled flag (1 = monitor)
} target_ports SEC(".maps");

// Per-port aggregate statistics (renamed to avoid kernel conflict)
struct pm_port_stats {
  __u64 rx_bytes;
  __u64 tx_bytes;
  __u64 rx_packets;
  __u64 tx_packets;
  __u64 connections;
};

struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 64);
  __type(key, __u16);
  __type(value, struct pm_port_stats);
} port_stats_map SEC(".maps");

// Per-connection key for tracking individual connections
struct pm_conn_key {
  __u32 saddr;
  __u32 daddr;
  __u16 sport;
  __u16 dport;
};

// Per-connection statistics
struct pm_conn_stats {
  __u64 rx_bytes;
  __u64 tx_bytes;
  __u64 start_ns;
  __u64 last_update_ns;
};

struct {
  __uint(type, BPF_MAP_TYPE_HASH);
  __uint(max_entries, 10240);
  __type(key, struct pm_conn_key);
  __type(value, struct pm_conn_stats);
} conn_stats_map SEC(".maps");

// ============================================================================
// Helper Functions
// ============================================================================

// Check if a port is in our target list
static __always_inline int is_target_port(__u16 port) {
  __u8 *enabled = bpf_map_lookup_elem(&target_ports, &port);
  return enabled != NULL && *enabled;
}

// Get or create port stats entry
static __always_inline struct pm_port_stats *get_port_stats(__u16 port) {
  struct pm_port_stats *ps = bpf_map_lookup_elem(&port_stats_map, &port);
  if (!ps) {
    struct pm_port_stats zero = {};
    bpf_map_update_elem(&port_stats_map, &port, &zero, BPF_NOEXIST);
    ps = bpf_map_lookup_elem(&port_stats_map, &port);
  }
  return ps;
}

// ============================================================================
// Kprobe: tcp_sendmsg - Track outgoing TCP data
// ============================================================================

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_sendmsg, struct sock *sk, struct msghdr *msg,
               __u64 size) {
  if (!sk || size == 0) {
    return 0;
  }

  // Read socket addresses and ports
  __u16 sport = 0, dport = 0;
  __u32 saddr = 0, daddr = 0;

  BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);
  BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
  BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
  BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);

  // Convert dport from network byte order
  dport = bpf_ntohs(dport);

  // Check if either port is monitored
  __u16 target = 0;
  if (is_target_port(sport)) {
    target = sport;
  } else if (is_target_port(dport)) {
    target = dport;
  } else {
    return 0; // Not a monitored port
  }

  // Update port-level statistics (TX - sending data)
  struct pm_port_stats *ps = get_port_stats(target);
  if (ps) {
    __sync_fetch_and_add(&ps->tx_bytes, size);
    __sync_fetch_and_add(&ps->tx_packets, 1);
  }

  // Update per-connection statistics
  struct pm_conn_key ck = {
      .saddr = saddr,
      .daddr = daddr,
      .sport = sport,
      .dport = dport,
  };

  __u64 now = bpf_ktime_get_ns();
  struct pm_conn_stats *cs = bpf_map_lookup_elem(&conn_stats_map, &ck);
  if (cs) {
    __sync_fetch_and_add(&cs->tx_bytes, size);
    cs->last_update_ns = now;
  } else {
    struct pm_conn_stats new_cs = {
        .tx_bytes = size,
        .start_ns = now,
        .last_update_ns = now,
    };
    bpf_map_update_elem(&conn_stats_map, &ck, &new_cs, BPF_ANY);

    // Increment connection count for the target port
    if (ps) {
      __sync_fetch_and_add(&ps->connections, 1);
    }
  }

  return 0;
}

// ============================================================================
// Kprobe: tcp_cleanup_rbuf - Track incoming TCP data (more accurate than
// tcp_recvmsg)
// ============================================================================

SEC("kprobe/tcp_cleanup_rbuf")
int BPF_KPROBE(trace_tcp_cleanup_rbuf, struct sock *sk, int copied) {
  if (!sk || copied <= 0) {
    return 0;
  }

  // Read socket addresses and ports
  __u16 sport = 0, dport = 0;
  __u32 saddr = 0, daddr = 0;

  BPF_CORE_READ_INTO(&sport, sk, __sk_common.skc_num);
  BPF_CORE_READ_INTO(&dport, sk, __sk_common.skc_dport);
  BPF_CORE_READ_INTO(&saddr, sk, __sk_common.skc_rcv_saddr);
  BPF_CORE_READ_INTO(&daddr, sk, __sk_common.skc_daddr);

  // Convert dport from network byte order
  dport = bpf_ntohs(dport);

  // Check if either port is monitored
  __u16 target = 0;
  if (is_target_port(sport)) {
    target = sport;
  } else if (is_target_port(dport)) {
    target = dport;
  } else {
    return 0; // Not a monitored port
  }

  // Update port-level statistics (RX - receiving data)
  struct pm_port_stats *ps = get_port_stats(target);
  if (ps) {
    __sync_fetch_and_add(&ps->rx_bytes, (__u64)copied);
    __sync_fetch_and_add(&ps->rx_packets, 1);
  }

  // Update per-connection statistics
  struct pm_conn_key ck = {
      .saddr = saddr,
      .daddr = daddr,
      .sport = sport,
      .dport = dport,
  };

  __u64 now = bpf_ktime_get_ns();
  struct pm_conn_stats *cs = bpf_map_lookup_elem(&conn_stats_map, &ck);
  if (cs) {
    __sync_fetch_and_add(&cs->rx_bytes, (__u64)copied);
    cs->last_update_ns = now;
  } else {
    struct pm_conn_stats new_cs = {
        .rx_bytes = (__u64)copied,
        .start_ns = now,
        .last_update_ns = now,
    };
    bpf_map_update_elem(&conn_stats_map, &ck, &new_cs, BPF_ANY);

    // Increment connection count for the target port
    if (ps) {
      __sync_fetch_and_add(&ps->connections, 1);
    }
  }

  return 0;
}
