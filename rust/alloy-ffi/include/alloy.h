#ifndef ALLOY_H
#define ALLOY_H

#include <stddef.h>
#include <stdint.h>

#define ALLOY_OK 0
#define ALLOY_CLOSED 1

typedef struct AlloyComponent alloy_component;

typedef struct {
  uint8_t *ptr;
  size_t len;
  size_t cap;
} alloy_buf;

alloy_component *alloy_component_new(const char *name, const void *args,
                                     size_t args_size, alloy_buf *err);
void alloy_component_close(const alloy_component *c);
void alloy_component_free(alloy_component *c);

void alloy_source_run(const alloy_component *c);
int32_t alloy_source_recv(const alloy_component *c, alloy_buf *out);

void alloy_buf_free(alloy_buf *buf);

#endif
