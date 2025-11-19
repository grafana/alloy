// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Example application that intentionally breaks its stack.
// 
// cc -O2 -g -o brokenstack brokenstack.c

#include <stdint.h>

#define FORCE_FRAME \
  __attribute__((noinline)) \
  __attribute__((optimize("no-omit-frame-pointer"))) \
  __attribute__((optimize("no-optimize-sibling-calls")))

static volatile int cond = 1;

FORCE_FRAME void a() {
  while(cond);
}

FORCE_FRAME void b() {
  a();
}

FORCE_FRAME void c() {
  uint64_t* frame = __builtin_frame_address(0);
  frame[0] = 0x42;
  frame[1] = 0x42;
  b();
}

int main() {
  c();
}

