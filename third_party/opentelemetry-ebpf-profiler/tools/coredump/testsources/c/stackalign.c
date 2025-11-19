// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// gcc -O3 -fomit-frame-pointer -mavx -ftree-vectorize stackalign.c -o stackalign

#include <unistd.h>
#include <stdio.h>

int calc(int r)
{
	const int N = 2000; //Array Size
	const int noTests = 10000; //Number of tests
	float a[N],b[N],c[N],result[N];

	for (int i = 0; i < N; ++i) {
		a[i] =       ((float)i)+ 0.1335f;
		b[i] = 1.50f*((float)i)+ 0.9383f;
		c[i] = 0.33f*((float)i)+ 0.1172f;
	}

	for (int i = 0; i < noTests; ++i)
		for (int j = 0; j < N; ++j)
			result[j] = a[j]+b[j]-c[j]+3*(float)i;

	while (r == 3) pause();

	calc(r+1);

	fprintf(stderr, "calc(%d) done\n", r);
}

int main(void)
{
	calc(0);
	return 0;
}
