// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include <stdlib.h>
#include <dlfcn.h>

typedef int(*run_fake_apm_agent_t)();

int main() {
  // NOTE: `./` is necessary to make dlopen consider searching in local paths.
  void* handle = dlopen("./" LIB_NAME, RTLD_LAZY);
  if (!handle) {
    return 101;
  }

  run_fake_apm_agent_t fn = dlsym(handle, "run_fake_apm_agent");
  if (dlerror() != NULL) {
    return 102;
  }

  return fn();
}
