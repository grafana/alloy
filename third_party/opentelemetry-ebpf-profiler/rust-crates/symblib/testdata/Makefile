.PHONY: inline inline-compressed-dwarf inline-split-dwarf inline-big-fake-compressed-dwarf \
	inline-no-tco clean inline-compressed-dwarf-zstd go-toolchains

all: inline inline-compressed-dwarf inline-split-dwarf inline-big-fake-compressed-dwarf \
	inline-no-tco inline-compressed-dwarf-zstd go-toolchains

inline: inline.c
	cc $< -o $@ -O2 -g

inline-no-tco: inline.c
	cc $< -o $@ -O2 -g -fno-omit-frame-pointer -fno-optimize-sibling-calls
	
inline-compressed-dwarf: inline
	objcopy --compress-debug-sections=zlib $< $@

inline-compressed-dwarf-zstd: inline
	objcopy --compress-debug-sections=zstd $< $@

inline-big-fake-compressed-dwarf: inline
	dd if=/dev/zero bs=4M count=16 of=/tmp/big-fake-dwarf
	# objcopy only supports compressing DWARF sections, not arbitrary ones,
	# so we swap a DWARF section here to work around that limitation
	objcopy --update-section .debug_info=/tmp/big-fake-dwarf $< $@
	objcopy --compress-debug-sections $@

inline-split-dwarf: inline
	cp inline inline-split-dwarf
	dwz -M meow -m inline-split-dwarf.dwp inline-split-dwarf inline-split-dwarf

go-toolchains:
	GOTOOLCHAIN=go1.20.14 go build -o go-1.20.14 main.go
	GOTOOLCHAIN=go1.22.12 go build -o go-1.22.12 main.go
	GOTOOLCHAIN=go1.24.0 go build -o go-1.24.0 main.go

clean:
	echo "not deleting anything: executables are meant to be kept under VC"
