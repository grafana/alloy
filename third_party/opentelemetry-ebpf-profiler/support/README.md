This directory is intended for non-Go support functionality. For example, the
eBPF code and supporting Python scripts for bytecode offsets to line number
translation.

## Testing eBPF code on different kernel version
Via the following commands, you can run the eBPF loading tests on kernel version
5.4.276 or 6.12.16 respectively.
```
$ ./run-tests.sh 5.4.276
$ ./run-tests.sh 6.12.16
```
The script loads the provided eBPF code into the kernel in a virtual environment so that it does not affect your local environment.

## Requirements
The tests are built on top of the following dependencies. Make sure you have them installed beforehand.

 * qemu-system-x86
 * statically linked busybox

 ## Building a Custom Kernel Image
 Kernel images can be build with the script provided in `ci-kernels`. This directory contains also the basic configuration settings needed to enable eBPF features for the kernel image.

 ## Test a Custom Kernel Image
 By default `run-tests.sh` takes only the kernel version as argument. The script looks for the kernel image with the specified version in `ci-kernels`. As an alternative one can provide a directory to look for this kernel image via `KERN_DIR`.
 ```
 $ KERN_DIR=my-other-kernels/ ./run-tests.sh 5.4.276
 ```

 ## Manually Debugging a Custom Kernel Image
1. Compile eBPF and Go code
```
$ make ebpf
$ cd support
$ go test -c -tags integration ./...
```
2. Get [virtme](https://git.kernel.org/pub/scm/utils/kernel/virtme/virtme.git/) to run the environment
```
$ tmp_virtme="$(mktemp -d --suffix=-virtme)"
$ git clone -q https://git.kernel.org/pub/scm/utils/kernel/virtme/virtme.git "${tmp_virtme}"
```
3. Start the virtual environment for debugging with gdb:
```
$ ${tmp_virtme}/virtme-run --kimg  ci-kernels/linux-5.4.276.bz \
    --memory 4096M \
    --pwd \
    --script-sh "mount -t bpf bpf /sys/fs/bpf ; ./support.test -test.v" \
    --qemu-opts -append nokaslr -s
```
4. Start gdb in a second shell:
```
$ cd support
$ gdb
# Attach gdb to the running qemu process in the same directory:
(gdb) target remote localhost:1234
# Load source code:
(gdb) directory ./ci-kernels/_build/linux-5.4.276
# Load symbols for debugging:
(gdb) sym ./ci-kernels/_build/linux-5.4.276/vmlinux
# Set breakpoint at entry of eBPF verifier:
(gdb) break do_check
Breakpoint 1 at 0xffffffff81184460: file kernel/bpf/verifier.c, line 4105.
```
