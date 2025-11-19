// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include <errno.h>
#include <inttypes.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <threads.h>
#include <time.h>
#include <unistd.h>

#define PACKED __attribute__((packed))

static const char* SERVICE_NAME = "fake-apm-service";
static const char* SERVICE_ENV = "fake-apm-env";
static const char* SOCK_PATH_FMT = "/tmp/apm-corr-test-socket-%d";

thread_local void* elastic_apm_profiling_correlation_tls_v1 = NULL;
void* elastic_apm_profiling_correlation_process_storage_v1 = NULL;

typedef union ID128 {
  uint8_t raw[16];
  struct {
    uint64_t lo;
    uint64_t hi;
  } as_int;
} ID128;

typedef union ID64 {
  uint8_t raw[8];
  uint64_t as_int;
} ID64;

typedef ID64 ApmSpanID;

typedef ID128 ApmTraceID;

typedef ID128 UPTraceID;

typedef struct PACKED ApmCorrelationBuf {
  uint16_t layout_minor_version;
  uint8_t valid;
  uint8_t trace_present;
  uint8_t trace_flags;
  ApmTraceID trace_id;
  ApmSpanID span_id;
  ApmSpanID tx_id;
} ApmCorrelationBuf;

typedef struct PACKED ApmSocketMessage {
  uint16_t message_type;
  uint16_t minor_version;
  ApmTraceID apm_trace_id;
  ApmSpanID apm_tx_id;
  UPTraceID up_trace_id;
  uint16_t count;
} ApmSocketMessage;

static void* fake_java_apm_agent_recv_thread(void* fd_ptr) {
  int fd = *(int*)fd_ptr;
  
  for (;;) {
    ApmSocketMessage msg = {0};
    int n = recv(fd, &msg, sizeof msg, 0);

    if (n != sizeof msg) {
      printf("Received truncated message (%d bytes)\n", n);
      continue;
    }

    printf(
      "Received trace mapping from profiler:\n"
      "  APM trace ID: %016" PRIX64 ":%016" PRIX64 "\n"
      "  APM TX:       %016" PRIX64 "\n"
      "  UP trace ID:  %016" PRIX64 ":%016" PRIX64 "\n"
      "  Sample count: %" PRIu16 "\n\n",
      msg.apm_trace_id.as_int.hi,
      msg.apm_trace_id.as_int.lo,
      msg.apm_tx_id.as_int,
      msg.up_trace_id.as_int.hi,
      msg.up_trace_id.as_int.lo,
      msg.count
    );
  }

  return NULL;
}

static ApmCorrelationBuf* alloc_correlation_buf(ApmSpanID tx_id) {
  ApmCorrelationBuf* corr_buf = malloc(sizeof(ApmCorrelationBuf));

  *corr_buf = (ApmCorrelationBuf) {
    .layout_minor_version = 1,
    .valid = 1,
    .trace_present = 1,
    .trace_flags = 0,
    .trace_id.raw = {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
    .span_id.raw = {0, 1, 2, 3, 4, 5, 6, 7},
    .tx_id = tx_id
  };

  return corr_buf;
}

static void put_str(uint8_t** write_ptr, const char* str) {
  uint32_t len = strlen(str);
  memcpy(*write_ptr, &len, sizeof len);
  *write_ptr += sizeof len;
  memcpy(*write_ptr, str, (size_t)len);
  *write_ptr += len;
}

int run_fake_apm_agent() {
  //
  // Create and bind socket
  //

  int fd = socket(PF_UNIX, SOCK_DGRAM, 0);
  if (fd == -1) {
    return 1;
  }

  struct sockaddr_un addr = { .sun_family = AF_UNIX };
  srand(time(NULL));
  int n = snprintf(addr.sun_path, sizeof addr.sun_path, SOCK_PATH_FMT, rand());
  if (n > sizeof addr.sun_path) {
    return 2;
  }

  if (unlink(addr.sun_path) == -1 && errno != ENOENT) {
    return 3;
  }

  if (bind(fd, (struct sockaddr*)&addr, sizeof addr) < 0) {
    return 4;
  }

  //
  // Allocate and populate two correlation buffers.
  //

  ApmCorrelationBuf* corr_buf_1 = alloc_correlation_buf(
    (ApmSpanID) { .as_int = 0x0011223344556677ULL }
  );
  ApmCorrelationBuf* corr_buf_2 = alloc_correlation_buf(
    (ApmSpanID) { .as_int = 0x8899AABBCCDDEEFFULL }
  );

  //
  // Allocate and populate process storage
  //

  uint8_t* process_storage = malloc(256);
  uint8_t* write_ptr = process_storage;

  *(uint16_t*)write_ptr = 1; // Layout minor version
  write_ptr += sizeof(uint16_t);

  put_str(&write_ptr, SERVICE_NAME);
  put_str(&write_ptr, SERVICE_ENV);
  put_str(&write_ptr, addr.sun_path);

  elastic_apm_profiling_correlation_process_storage_v1 = process_storage;

  //
  // Spawn thread reading messages from the socket
  //

  pthread_t thread;
  pthread_attr_t thread_attr;
  pthread_attr_init(&thread_attr);
  if (pthread_create(&thread, &thread_attr, fake_java_apm_agent_recv_thread, &fd)) {
    return 5;
  }

  //
  // Generate samples by spinning in an infinite loop.
  //

  printf("Socket bound to `%s`. Spinning & waiting for messages from UP.\n", addr.sun_path);

  for (uint16_t x; ; ++x) {
    switch (x) {
      case 0: elastic_apm_profiling_correlation_tls_v1 = corr_buf_1; break;
      case 1 << 15: elastic_apm_profiling_correlation_tls_v1 = corr_buf_2; break;
    }
  }

  return 0;
}
