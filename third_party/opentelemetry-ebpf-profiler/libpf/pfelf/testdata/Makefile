.PHONY: all

CC ?= cc
OBJCOPY ?= objcopy

BINARIES=fixed-address \
	go-binary \
	kernel-image \
	separate-debug-file \
	the_notorious_build_id \
	ubuntu-kernel-image \
	with-debug-syms \
	without-debug-syms

all: $(BINARIES)

clean:
	rm -f $(BINARIES)

with-debug-syms: test.c
	$(CC) $< -g -o $@

without-debug-syms: test.c
	$(CC) $< -s -o $@

separate-debug-file: with-debug-syms
	$(OBJCOPY) --only-keep-debug $< $@

fixed-address: fixed-address.c fixed-address.ld
	# The following command will likely print a warning (about a missing -T option), which should be ignored.
	# Removing the warning would require passing a fully-fledged linker script to bypass gcc's default.
	$(CC) $^ -o $@

# Write an ELF notes file with a build ID
the_notorious_build_id:
	# \x04\x00\x00\x00: little endian for 4: the length of "GNU" + null character
	# \x14\x00\x00\x00: little endian for 20: the length of "_notorious_build_id_"
	# \x03\x00\x00\x00: little endian for 0x3 (Build ID note)
	bash -c "echo -en 'somedata\x04\x00\x00\x00\x14\x00\x00\x00\x03\x00\x00\x00GNU\x00_notorious_build_id_\x00somedata' > $@"

kernel-image: test.c
	$(CC) $< -s -o $@ -DLINUX_VERSION="\"Linux version 1.2.3\\n\""

ubuntu-kernel-image: test.c
	$(CC) $< -s -o $@ -DLINUX_VERSION="\"Linux version 1.2.3 (Ubuntu 4.5.6)\\n\""

go-binary: gotest.go
	go build -o go-binary -ldflags "-w -s" gotest.go

