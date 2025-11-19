// Defines types and constants that would usually come from kernel headers.
//
// Configuring include paths for kernel headers in a fashion that works across
// distributions is a pain and, even if it does work, tends to break again after
// a few updates. We only need a handful of those types anyway, so we just define
// them ourselves and ditch the problem entirely.

#ifndef OPTI_KERNEL_H
#define OPTI_KERNEL_H

// Define kernel integer types.
//
// One might be inclined to use gcc/clang built-in types like __INT32_TYPE__ to
// do this without relying on the (arch-dependent!) sizes of the primitive
// integer types, but this ends up confusing clang's format string validation.
// For example if `u64` is defined as `__UINT64_TYPE__`, clang will complain if
// it is used with `%lld` because there exist archs where `sizeof(long long)`
// is not the same as `sizeof(__UINT64_TYPE__)`.
//
// We thus define the integer types based on primitive types and do a static
// assertion that this works as expected. All of our target architectures use
// the same integer model, so there's no need for ifdefs.
typedef signed char s8;
typedef unsigned char u8;
typedef signed short s16;
typedef unsigned short u16;
typedef signed int s32;
typedef unsigned int u32;
typedef signed long long s64;
typedef unsigned long long u64;

// Size types need to be declared with __SIZE_TYPE__ built-in to not clash with
// CGo's built-in prolog.
typedef __SIZE_TYPE__ uintptr_t;
typedef __SIZE_TYPE__ size_t;

_Static_assert(sizeof(s8) == 1, "bad s8 size");
_Static_assert(sizeof(u8) == 1, "bad u8 size");
_Static_assert(sizeof(s16) == 2, "bad s16 size");
_Static_assert(sizeof(u16) == 2, "bad u16 size");
_Static_assert(sizeof(s32) == 4, "bad s32 size");
_Static_assert(sizeof(u32) == 4, "bad u32 size");
_Static_assert(sizeof(s64) == 8, "bad s64 size");
_Static_assert(sizeof(u64) == 8, "bad u64 size");

_Static_assert(sizeof(uintptr_t) == 8, "bad uintptr_t size");
_Static_assert(sizeof(size_t) == 8, "bad size_t size");

// Define bool type (emulates stdbool.h).
#if __STDC_VERSION__ < 202311L
typedef _Bool bool;
  #ifndef __bool_true_false_are_defined
    #define true                          1
    #define false                         0
    #define __bool_true_false_are_defined 1
  #endif
#endif

// Go defines `NULL` in `cgo-builtin-prolog`, so we have to check whether
// it is already defined here (for when this is included in CGo).
#ifndef NULL
  #define NULL ((void *)0)
#endif

typedef int pid_t;

typedef u32 __be32;
typedef u64 __be64;

#define ATOMIC_ADD(ptr, n) __sync_fetch_and_add(ptr, n)

struct task_struct;

// Defined in arch/{x86,arm64}/include/asm/ptrace.h
#if defined(__x86_64)
  #define reg_pc ip
struct pt_regs {
  unsigned long r15;
  unsigned long r14;
  unsigned long r13;
  unsigned long r12;
  unsigned long bp;
  unsigned long bx;
  unsigned long r11;
  unsigned long r10;
  unsigned long r9;
  unsigned long r8;
  unsigned long ax;
  unsigned long cx;
  unsigned long dx;
  unsigned long si;
  unsigned long di;
  unsigned long orig_ax;
  unsigned long ip;
  unsigned long cs;
  unsigned long flags;
  unsigned long sp;
  unsigned long ss;
};
#elif defined(__aarch64__)
struct pt_regs {
  u64 regs[31];
  u64 sp;
  u64 pc;
  u64 pstate;
  u64 orig_x0;
  s32 syscallno;
  u32 unused2;
  u64 sdei_ttbr1;
  u64 pmr_save;
  u64 stackframe[2];
  u64 lockdep_hardirqs;
  u64 exit_rcu;
};
  #define reg_pc pc
#else
  #error "Unsupported architecture"
#endif

struct bpf_perf_event_data {
  struct pt_regs regs;
};

// The following works with clang and gcc.
// Checked with
//    clang -dM -E -x c /dev/null | grep ENDI
//      gcc -dM -E -x c /dev/null | grep ENDI
#if defined __BYTE_ORDER__ && __BYTE_ORDER__ == __ORDER_LITTLE_ENDIAN__
  #define __constant_cpu_to_be32(x) __builtin_bswap32(x)
  #define __constant_cpu_to_be64(x) __builtin_bswap64(x)
#elif defined __BYTE_ORDER__ && __BYTE_ORDER__ == __ORDER_BIG_ENDIAN__
  #define __constant_cpu_to_be32(x) (x)
  #define __constant_cpu_to_be64(x) (x)
#else
  #error "Unknown endianness"
#endif

struct bpf_raw_tracepoint_args {
  u64 args[0];
};

// Flags for bpf_map_update_elem
enum {
  BPF_ANY     = 0,
  BPF_NOEXIST = 1,
  BPF_EXIST   = 2,
  BPF_F_LOCK  = 4,
};

// Flags for perf event helpers
enum {
  BPF_F_INDEX_MASK  = 0xFFFFFFFFULL,
  BPF_F_CURRENT_CPU = BPF_F_INDEX_MASK,
  BPF_F_CTXLEN_MASK = (0xFFFFFULL << 32),
};

// BPF map variants.
enum bpf_map_type {
  BPF_MAP_TYPE_UNSPEC,
  BPF_MAP_TYPE_HASH,
  BPF_MAP_TYPE_ARRAY,
  BPF_MAP_TYPE_PROG_ARRAY,
  BPF_MAP_TYPE_PERF_EVENT_ARRAY,
  BPF_MAP_TYPE_PERCPU_HASH,
  BPF_MAP_TYPE_PERCPU_ARRAY,
  BPF_MAP_TYPE_STACK_TRACE,
  BPF_MAP_TYPE_CGROUP_ARRAY,
  BPF_MAP_TYPE_LRU_HASH,
  BPF_MAP_TYPE_LRU_PERCPU_HASH,
  BPF_MAP_TYPE_LPM_TRIE,
  BPF_MAP_TYPE_ARRAY_OF_MAPS,
  BPF_MAP_TYPE_HASH_OF_MAPS,
  BPF_MAP_TYPE_DEVMAP,
  BPF_MAP_TYPE_SOCKMAP,
  BPF_MAP_TYPE_CPUMAP,
  BPF_MAP_TYPE_XSKMAP,
  BPF_MAP_TYPE_SOCKHASH,
  BPF_MAP_TYPE_CGROUP_STORAGE_DEPRECATED,
  BPF_MAP_TYPE_CGROUP_STORAGE = BPF_MAP_TYPE_CGROUP_STORAGE_DEPRECATED,
  BPF_MAP_TYPE_REUSEPORT_SOCKARRAY,
  BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE,
  BPF_MAP_TYPE_QUEUE,
  BPF_MAP_TYPE_STACK,
  BPF_MAP_TYPE_SK_STORAGE,
  BPF_MAP_TYPE_DEVMAP_HASH,
  BPF_MAP_TYPE_STRUCT_OPS,
  BPF_MAP_TYPE_RINGBUF,
  BPF_MAP_TYPE_INODE_STORAGE,
  BPF_MAP_TYPE_TASK_STORAGE,
  BPF_MAP_TYPE_BLOOM_FILTER,
  BPF_MAP_TYPE_USER_RINGBUF,
  BPF_MAP_TYPE_CGRP_STORAGE,
};

// Flags bpf_get_stackid/bpf_get_stack.
enum {
  BPF_F_SKIP_FIELD_MASK = 0xffULL,
  BPF_F_USER_STACK      = (1ULL << 8),
  BPF_F_FAST_STACK_CMP  = (1ULL << 9),
  BPF_F_REUSE_STACKID   = (1ULL << 10),
  BPF_F_USER_BUILD_ID   = (1ULL << 11),
};

// Flags for bpf_map_create
enum {
  BPF_F_NO_PREALLOC = (1U << 0),
  // (other values omitted here)
};

// BPF helper function IDs
// https://github.com/torvalds/linux/blob/e8f897f4a/include/uapi/linux/bpf.h#L5683
#define BPF_FUNC_map_lookup_elem                1
#define BPF_FUNC_map_update_elem                2
#define BPF_FUNC_map_delete_elem                3
#define BPF_FUNC_probe_read                     4
#define BPF_FUNC_ktime_get_ns                   5
#define BPF_FUNC_trace_printk                   6
#define BPF_FUNC_get_prandom_u32                7
#define BPF_FUNC_get_smp_processor_id           8
#define BPF_FUNC_skb_store_bytes                9
#define BPF_FUNC_l3_csum_replace                10
#define BPF_FUNC_l4_csum_replace                11
#define BPF_FUNC_tail_call                      12
#define BPF_FUNC_clone_redirect                 13
#define BPF_FUNC_get_current_pid_tgid           14
#define BPF_FUNC_get_current_uid_gid            15
#define BPF_FUNC_get_current_comm               16
#define BPF_FUNC_get_cgroup_classid             17
#define BPF_FUNC_skb_vlan_push                  18
#define BPF_FUNC_skb_vlan_pop                   19
#define BPF_FUNC_skb_get_tunnel_key             20
#define BPF_FUNC_skb_set_tunnel_key             21
#define BPF_FUNC_perf_event_read                22
#define BPF_FUNC_redirect                       23
#define BPF_FUNC_get_route_realm                24
#define BPF_FUNC_perf_event_output              25
#define BPF_FUNC_skb_load_bytes                 26
#define BPF_FUNC_get_stackid                    27
#define BPF_FUNC_csum_diff                      28
#define BPF_FUNC_skb_get_tunnel_opt             29
#define BPF_FUNC_skb_set_tunnel_opt             30
#define BPF_FUNC_skb_change_proto               31
#define BPF_FUNC_skb_change_type                32
#define BPF_FUNC_skb_under_cgroup               33
#define BPF_FUNC_get_hash_recalc                34
#define BPF_FUNC_get_current_task               35
#define BPF_FUNC_probe_write_user               36
#define BPF_FUNC_current_task_under_cgroup      37
#define BPF_FUNC_skb_change_tail                38
#define BPF_FUNC_skb_pull_data                  39
#define BPF_FUNC_csum_update                    40
#define BPF_FUNC_set_hash_invalid               41
#define BPF_FUNC_get_numa_node_id               42
#define BPF_FUNC_skb_change_head                43
#define BPF_FUNC_xdp_adjust_head                44
#define BPF_FUNC_probe_read_str                 45
#define BPF_FUNC_get_socket_cookie              46
#define BPF_FUNC_get_socket_uid                 47
#define BPF_FUNC_set_hash                       48
#define BPF_FUNC_setsockopt                     49
#define BPF_FUNC_skb_adjust_room                50
#define BPF_FUNC_redirect_map                   51
#define BPF_FUNC_sk_redirect_map                52
#define BPF_FUNC_sock_map_update                53
#define BPF_FUNC_xdp_adjust_meta                54
#define BPF_FUNC_perf_event_read_value          55
#define BPF_FUNC_perf_prog_read_value           56
#define BPF_FUNC_getsockopt                     57
#define BPF_FUNC_override_return                58
#define BPF_FUNC_sock_ops_cb_flags_set          59
#define BPF_FUNC_msg_redirect_map               60
#define BPF_FUNC_msg_apply_bytes                61
#define BPF_FUNC_msg_cork_bytes                 62
#define BPF_FUNC_msg_pull_data                  63
#define BPF_FUNC_bind                           64
#define BPF_FUNC_xdp_adjust_tail                65
#define BPF_FUNC_skb_get_xfrm_state             66
#define BPF_FUNC_get_stack                      67
#define BPF_FUNC_skb_load_bytes_relative        68
#define BPF_FUNC_fib_lookup                     69
#define BPF_FUNC_sock_hash_update               70
#define BPF_FUNC_msg_redirect_hash              71
#define BPF_FUNC_sk_redirect_hash               72
#define BPF_FUNC_lwt_push_encap                 73
#define BPF_FUNC_lwt_seg6_store_bytes           74
#define BPF_FUNC_lwt_seg6_adjust_srh            75
#define BPF_FUNC_lwt_seg6_action                76
#define BPF_FUNC_rc_repeat                      77
#define BPF_FUNC_rc_keydown                     78
#define BPF_FUNC_skb_cgroup_id                  79
#define BPF_FUNC_get_current_cgroup_id          80
#define BPF_FUNC_get_local_storage              81
#define BPF_FUNC_sk_select_reuseport            82
#define BPF_FUNC_skb_ancestor_cgroup_id         83
#define BPF_FUNC_sk_lookup_tcp                  84
#define BPF_FUNC_sk_lookup_udp                  85
#define BPF_FUNC_sk_release                     86
#define BPF_FUNC_map_push_elem                  87
#define BPF_FUNC_map_pop_elem                   88
#define BPF_FUNC_map_peek_elem                  89
#define BPF_FUNC_msg_push_data                  90
#define BPF_FUNC_msg_pop_data                   91
#define BPF_FUNC_rc_pointer_rel                 92
#define BPF_FUNC_spin_lock                      93
#define BPF_FUNC_spin_unlock                    94
#define BPF_FUNC_sk_fullsock                    95
#define BPF_FUNC_tcp_sock                       96
#define BPF_FUNC_skb_ecn_set_ce                 97
#define BPF_FUNC_get_listener_sock              98
#define BPF_FUNC_skc_lookup_tcp                 99
#define BPF_FUNC_tcp_check_syncookie            100
#define BPF_FUNC_sysctl_get_name                101
#define BPF_FUNC_sysctl_get_current_value       102
#define BPF_FUNC_sysctl_get_new_value           103
#define BPF_FUNC_sysctl_set_new_value           104
#define BPF_FUNC_strtol                         105
#define BPF_FUNC_strtoul                        106
#define BPF_FUNC_sk_storage_get                 107
#define BPF_FUNC_sk_storage_delete              108
#define BPF_FUNC_send_signal                    109
#define BPF_FUNC_tcp_gen_syncookie              110
#define BPF_FUNC_skb_output                     111
#define BPF_FUNC_probe_read_user                112
#define BPF_FUNC_probe_read_kernel              113
#define BPF_FUNC_probe_read_user_str            114
#define BPF_FUNC_probe_read_kernel_str          115
#define BPF_FUNC_tcp_send_ack                   116
#define BPF_FUNC_send_signal_thread             117
#define BPF_FUNC_jiffies64                      118
#define BPF_FUNC_read_branch_records            119
#define BPF_FUNC_get_ns_current_pid_tgid        120
#define BPF_FUNC_xdp_output                     121
#define BPF_FUNC_get_netns_cookie               122
#define BPF_FUNC_get_current_ancestor_cgroup_id 123
#define BPF_FUNC_sk_assign                      124
#define BPF_FUNC_ktime_get_boot_ns              125
#define BPF_FUNC_seq_printf                     126
#define BPF_FUNC_seq_write                      127
#define BPF_FUNC_sk_cgroup_id                   128
#define BPF_FUNC_sk_ancestor_cgroup_id          129
#define BPF_FUNC_ringbuf_output                 130
#define BPF_FUNC_ringbuf_reserve                131
#define BPF_FUNC_ringbuf_submit                 132
#define BPF_FUNC_ringbuf_discard                133
#define BPF_FUNC_ringbuf_query                  134
#define BPF_FUNC_csum_level                     135
#define BPF_FUNC_skc_to_tcp6_sock               136
#define BPF_FUNC_skc_to_tcp_sock                137
#define BPF_FUNC_skc_to_tcp_timewait_sock       138
#define BPF_FUNC_skc_to_tcp_request_sock        139
#define BPF_FUNC_skc_to_udp6_sock               140
#define BPF_FUNC_get_task_stack                 141
#define BPF_FUNC_load_hdr_opt                   142
#define BPF_FUNC_store_hdr_opt                  143
#define BPF_FUNC_reserve_hdr_opt                144
#define BPF_FUNC_inode_storage_get              145
#define BPF_FUNC_inode_storage_delete           146
#define BPF_FUNC_d_path                         147
#define BPF_FUNC_copy_from_user                 148
#define BPF_FUNC_snprintf_btf                   149
#define BPF_FUNC_seq_printf_btf                 150
#define BPF_FUNC_skb_cgroup_classid             151
#define BPF_FUNC_redirect_neigh                 152
#define BPF_FUNC_per_cpu_ptr                    153
#define BPF_FUNC_this_cpu_ptr                   154
#define BPF_FUNC_redirect_peer                  155
#define BPF_FUNC_task_storage_get               156
#define BPF_FUNC_task_storage_delete            157
#define BPF_FUNC_get_current_task_btf           158
#define BPF_FUNC_bprm_opts_set                  159
#define BPF_FUNC_ktime_get_coarse_ns            160
#define BPF_FUNC_ima_inode_hash                 161
#define BPF_FUNC_sock_from_file                 162
#define BPF_FUNC_check_mtu                      163
#define BPF_FUNC_for_each_map_elem              164
#define BPF_FUNC_snprintf                       165
#define BPF_FUNC_sys_bpf                        166
#define BPF_FUNC_btf_find_by_name_kind          167
#define BPF_FUNC_sys_close                      168
#define BPF_FUNC_timer_init                     169
#define BPF_FUNC_timer_set_callback             170
#define BPF_FUNC_timer_start                    171
#define BPF_FUNC_timer_cancel                   172
#define BPF_FUNC_get_func_ip                    173
#define BPF_FUNC_get_attach_cookie              174
#define BPF_FUNC_task_pt_regs                   175
#define BPF_FUNC_get_branch_snapshot            176
#define BPF_FUNC_trace_vprintk                  177
#define BPF_FUNC_skc_to_unix_sock               178
#define BPF_FUNC_kallsyms_lookup_name           179
#define BPF_FUNC_find_vma                       180
#define BPF_FUNC_loop                           181
#define BPF_FUNC_strncmp                        182
#define BPF_FUNC_get_func_arg                   183
#define BPF_FUNC_get_func_ret                   184
#define BPF_FUNC_get_func_arg_cnt               185
#define BPF_FUNC_get_retval                     186
#define BPF_FUNC_set_retval                     187
#define BPF_FUNC_xdp_get_buff_len               188
#define BPF_FUNC_xdp_load_bytes                 189
#define BPF_FUNC_xdp_store_bytes                190
#define BPF_FUNC_copy_from_user_task            191
#define BPF_FUNC_skb_set_tstamp                 192
#define BPF_FUNC_ima_file_hash                  193
#define BPF_FUNC_kptr_xchg                      194
#define BPF_FUNC_map_lookup_percpu_elem         195
#define BPF_FUNC_skc_to_mptcp_sock              196
#define BPF_FUNC_dynptr_from_mem                197
#define BPF_FUNC_ringbuf_reserve_dynptr         198
#define BPF_FUNC_ringbuf_submit_dynptr          199
#define BPF_FUNC_ringbuf_discard_dynptr         200
#define BPF_FUNC_dynptr_read                    201
#define BPF_FUNC_dynptr_write                   202
#define BPF_FUNC_dynptr_data                    203
#define BPF_FUNC_tcp_raw_gen_syncookie_ipv4     204
#define BPF_FUNC_tcp_raw_gen_syncookie_ipv6     205
#define BPF_FUNC_tcp_raw_check_syncookie_ipv4   206
#define BPF_FUNC_tcp_raw_check_syncookie_ipv6   207
#define BPF_FUNC_ktime_get_tai_ns               208
#define BPF_FUNC_user_ringbuf_drain             209
#define BPF_FUNC_cgrp_storage_get               210
#define BPF_FUNC_cgrp_storage_delete            211

// defined in include/uapi/linux/perf_event.h
#define PERF_MAX_STACK_DEPTH 127

#endif // OPTI_KERNEL_H
