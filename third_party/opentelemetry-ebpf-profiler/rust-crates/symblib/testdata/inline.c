// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include <stdio.h>

#define NOINLINE __attribute__((noinline))
#define INLINE static inline __attribute__((always_inline))

NOINLINE int d() {
  printf("hello!\n");
}

NOINLINE int c() {
  d();
}

NOINLINE int b() {
  c();
}

NOINLINE int a() {
  b();
}

INLINE int d_inline() {
  printf("hello!\n");
}

INLINE int c_inline() {
  d_inline();
}

INLINE int b_inline() {
  c_inline();
}

INLINE int a_inline() {
  b_inline();
}

NOINLINE int main() {
  a();
  a_inline();
}
