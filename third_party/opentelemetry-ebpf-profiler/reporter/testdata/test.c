// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include <unistd.h>

int main(int argc, char *argv[]) {
    // This process must not return (tests depend on it)
	return pause();
}
