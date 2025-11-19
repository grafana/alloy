#include <time.h>

int main(void)
{
  struct timespec res;

  while (1) {
    clock_gettime(CLOCK_MONOTONIC, &res);
  }
}
