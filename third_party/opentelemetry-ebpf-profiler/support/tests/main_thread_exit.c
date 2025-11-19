// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

/*
 * Implements a profiling test based on a multi-threaded process with
 * a main thread that exits early.
 *
 * Two additional threads are created:
 *   1. Burns CPU, ensures process is sampled by the profiler
 *   2. Burns CPU in newly mapped pages
 *
 * After main thread exits, /proc/PID/maps is empty and the expected
 * behavior is for the profiler to not cleanup the process, but instead
 * keep profiling the remaining thread and use /proc/PID/task/TID/maps
 * (TID corresponding to thread 2) to synchronize mappings.
 *
 * Needs OpenSSL (libssl) installed as it dynamically loads libcrypto.so.
 */

#define _GNU_SOURCE
#include <dlfcn.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

static void *burn(void *arg)
{
  int old_type;

  // We're just burning CPU, so asynchronous cancellation is safe
  pthread_setcanceltype(PTHREAD_CANCEL_ASYNCHRONOUS, &old_type);

  for (;;) {
  }

  // Never reached
  return NULL;
}

static void *hash(void *arg)
{
  unsigned char buf[1024];

  printf("Thread TID: %d, sleeping for 5s\n", gettid());
  sleep(5);

  unsigned char *(*MD5)(const unsigned char *d, unsigned long n, unsigned char *md);
  void *handle = dlopen("libcrypto.so", RTLD_LAZY);
  if (!handle) {
    fprintf(stderr, "%s\n", dlerror());
    return NULL;
  }

  MD5 = dlsym(handle, "MD5");
  if (!MD5) {
    fprintf(stderr, "dlsym: Could not resolve MD5\n");
    return NULL;
  }

  printf("Thread TID: %d, hashing..\n", gettid());
  for (;;) {
    MD5(buf, sizeof(buf), NULL);
  }

  // Never reached
  return NULL;
}

int main()
{
  int ret;
  pthread_t tid;
  printf("Main thread is starting, PID: %d\n", getpid());

  // Create a new thread to burn CPU / ensure process gets profiled
  if ((ret = pthread_create(&tid, NULL, burn, NULL)) != 0) {
    fprintf(stderr, "pthread_create: burn %d\n", ret);
    exit(EXIT_FAILURE);
  }

  sleep(2);

  printf("Press ENTER to exit main thread: ");
  getchar();

  // Stop CPU burn thread to reduce noise while hash thread is running
  void *tret;
  pthread_cancel(tid);
  pthread_join(tid, &tret);

  if (tret != PTHREAD_CANCELED) {
    fprintf(stderr, "pthread_join: %p\n", tret);
    exit(EXIT_FAILURE);
  }

  printf("Main thread is exiting\n");

  // Create a new thread to burn CPU in newly mapped pages
  if ((ret = pthread_create(&tid, NULL, hash, NULL) != 0)) {
    fprintf(stderr, "pthread_create: hash %d\n", ret);
    exit(EXIT_FAILURE);
  }

  pthread_detach(pthread_self());
  pthread_exit(NULL);
  return 0;
}
