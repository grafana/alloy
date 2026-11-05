#ifndef RUSTGEN_H
#define RUSTGEN_H

#include <stddef.h>
#include <stdint.h>

#define RG_OK 0
#define RG_CLOSED 1

typedef struct Gen rg_gen;

typedef struct {
  uint8_t *ptr;
  size_t len;
  size_t cap;
} rg_buf;

rg_gen *rg_new(uint32_t lines_per_batch, uint64_t interval_ms);
int32_t rg_next_batch(const rg_gen *g, rg_buf *out);
void rg_buf_free(rg_buf *buf);
void rg_close(const rg_gen *g);
void rg_free(rg_gen *g);

#endif
