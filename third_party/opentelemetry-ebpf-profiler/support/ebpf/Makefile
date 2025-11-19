SHELL ?= bash
BPF_CLANG ?= clang-17
BPF_LINK ?= llvm-link-17
STRIP ?= llvm-strip-17
LLC ?= llc-17
CLANG_FORMAT ?= clang-format-17

# Detect native architecture and translate to GOARCH.
NATIVE_ARCH := $(shell uname -m)
ifeq ($(NATIVE_ARCH),x86_64)
NATIVE_ARCH := amd64
else ifneq (,$(filter $(NATIVE_ARCH),aarch64 arm64))
NATIVE_ARCH := arm64
else
$(error Unsupported architecture: $(NATIVE_ARCH))
endif

# Use a placeholder like '.' or '/' as the new prefix.
REPRODUCIBLE_PREFIX := .

# Valid values: amd64, arm64.
TARGET_ARCH ?= $(NATIVE_ARCH)

TRACER_NAME ?= tracer.ebpf.$(TARGET_ARCH)

ifeq ($(TARGET_ARCH),arm64)
TARGET_FLAGS = -target aarch64-linux-gnu
else
TARGET_FLAGS = -target x86_64-linux-gnu
endif

# Use -g to generate the btf section in the resulting binary.
FLAGS=$(TARGET_FLAGS) -g \
	-fno-jump-tables \
	-nostdlib \
	-nostdinc \
	-ffreestanding \
	-O2 -emit-llvm -c $< \
	-Wall -Wextra -Werror \
	-Wno-address-of-packed-member \
	-Wno-unused-label \
	-Wno-sign-compare \
	-fno-stack-protector \
	-D__SOURCE_DATE_EPOCH__=0 \
	-Xclang -fdebug-prefix-map=$(CURDIR)=$(REPRODUCIBLE_PREFIX) \
	-Xclang -fmacro-prefix-map=$(CURDIR)=$(REPRODUCIBLE_PREFIX)

SRCS := $(wildcard *.ebpf.c)
OBJS := $(SRCS:.c=.$(TARGET_ARCH).o)
DEPS := $(OBJS:.o=.d)

.DEFAULT_GOAL := all

all: $(TRACER_NAME)

amd64:
	$(MAKE) TARGET_ARCH=amd64

arm64:
	$(MAKE) TARGET_ARCH=arm64

errors.h: ../../tools/errors-codegen/errors.json
	go run ../../tools/errors-codegen/main.go bpf $@

%.ebpf.$(TARGET_ARCH).o: %.ebpf.c errors.h
	$(BPF_CLANG) $(FLAGS) -MMD -MP -o $@

$(TRACER_NAME): $(OBJS)
	$(BPF_LINK) $^ -o - | $(LLC) -march=bpf -mcpu=v2 -filetype=obj -o $@
	# With the compile flag -g not only the btf section is added to the
	# binary but also additional debug sections. As these debug sections
	# are not relevant for BPF and increase the binary size remove them here.
	$(STRIP) --strip-debug --enable-deterministic-archives $@
	@./print_instruction_count.sh $@

baseline: $(TRACER_NAME)
	cp $< $(TRACER_NAME).$@

bloatcheck: $(TRACER_NAME)
	python3 bloat-o-meter $(TRACER_NAME).baseline $(TRACER_NAME)

lint:
	$(CLANG_FORMAT) -Werror --dry-run -style=file *.[ch] ../../tools/coredump/*.[ch]

format:
	$(CLANG_FORMAT) -i -style=file *.[ch]

clean:
	rm -f *.o *.d

-include $(DEPS)
