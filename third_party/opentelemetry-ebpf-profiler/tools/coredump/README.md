coredump testing
================

A coredump is an ELF file of type `ET_CORE` that contains a full state of the
process including information about memory mappings, thread CPU states, etc.
Basically, it is a full snapshot of a process at a specific time.

In coredump testing, we compile the whole BPF unwinder code into a user-mode
executable, then use the information from a coredump to simulate a realistic
environment to test the unwinder code in. The coredump testing essentially
implements all required BPF helper functions in user-space, reading memory
and thread contexts from the coredump.

The primary intention here is to have solid regression test coverage of our
unwinding code, but another useful side effect is being able to single-step
through the unwinder code in `gdb`.

## Running the tests

The coredump test suite is run as part of the top level Makefile's "make tests"
or `go test ./...` from the repository's root.  All coredump test cases are 
automatically picked up, ran, and verified.

To run just the coredump tests without the remaining test suite – in this 
directory – run:

```bash
go test -v
```

To run an individual test, you can refer to it by its name:

```bash
go test -v -run TestCoreDumps/testdata/arm64/java.PrologueEpilogue.epi.add-sp-sp.377026.json
```

## Adding test cases

This section describes the steps and requirements to add coredump tests. Tests
can either be created directly using the `new` subcommand of the helper tool in
this directory or by manually creating a coredump and then importing it.

### Option 1: use `coredump new`

This is the most straight-forward way to create new test cases. It requires
that the `gcore` utility that usually ships with the `gdb` package is installed.
This approach automatically adjusts the coredump filter as required and ignores
the `ulimit`, so no further preparation is required.

When the application that you wish to create a test case for is in the desired
state, simply run:

```bash
./coredump new -pid $(pgrep my-app-name) -name my-test-case-name
```

Note that `coredump new` doesn't actually upload the coredump data to the remote
coredump storage -- please refer to the [dedicated section][upload] for more 
information.

[upload]: #uploading-test-case-data

If you run into issues mentioning `permission denied` you're probably lacking
privileges to debug the target process. In that case, simply run the command 
with prepended `sudo` and fix the owner of the files created by running 
`chown -R $UID:$GID .` in this directory.

### Option 2: import manually created coredump

We can also import a coredump that was previously created using one of
the options detailed in [the dedicated section][manually].

[manually]: #manually-creating-coredumps

```bash
./coredump new -core path/to/coredump -name my-test-case-name
```

**Important:** this will also import all ELF executables that were loaded when
the coredump was created by attempting to find them on disk at the path where
they were loaded at execution time. If this is incorrect, for example because
the coredump was created on a different system where you absolutely can't run
the `coredump` helper tool directly, you should pass `-no-module-bundling`.
This will make the coredump tests fall back to memory-dumping the required ELF
modules. It should generally be avoided because the environment presented to
the testee differs from what it will observe in the real world, but is still
preferable to bundling the wrong executables with the test case.

## Uploading test case data

To allow for local experiments without the need to upload a ton of data with
every attempt, `coredump new` does **not** automatically upload the data for the
test-case to S3. Once you are happy with your test case, you can push the data 
associated with the test case by running:

```bash
./coredump upload -all
```

You don't have to worry about this breaking anything on other branches: the
underlying storage solution ensures that your uploaded files will never clash
with existing test cases.

## Manually creating coredumps

### Option 1: make the kernel save a coredump

In this variant we essentially make the kernel think that the target application
crashed, causing the kernel to save a coredump for us.

#### Setting the coredump filter (optional)

Coredumps normally contain only the anonymous and modified pages to save disk
space. This is sufficient if the mapped in ELF files are available to the
`coredump` utility to be bundled. This is the case if you run
`./coredump new -core core` on the same machine where the core was generated,
or if you supply `-sysroot` as a prefix to find the correct files.

If the above is not possible, the testing infrastructure has limited support
to allow reading the ELF file data directly from the coredump. In this case
a full process memory dump that also contains the pages mapped into the process
from the ELF files is needed.

To get a full process memory dump one has to set the [`coredump_filter`][filter]
in advance by running:

[filter]: https://man7.org/linux/man-pages/man5/core.5.html

```bash
echo 0x3f > /proc/$PID/coredump_filter
```

**Note regarding PHP JIT:** if you want to add a PHP8+ coredump test you may 
need to set the filter to `0xff` instead. The reason for this is that PHP8+ 
uses shared pages for its JIT regions, and on some platforms like ARM64 the 
memory dump may not be able to capture this information.

#### Signals

The kernel will generate a coredump when a process is killed with a signal that
defaults to dumping core, and the system configuration allows coredump
generation. From the list of [suitable signals][signals]
`SIGILL` or `SIGSYS` are typically a good choice. Some VMs like Java's HotSpot
hook other signals such as `SIGBUS`, `SIGSEGV`, `SIGABRT` and handle them
internally. If a specific signal doesn't yield the expected result, simply
try a different one.

[signals]: https://man7.org/linux/man-pages/man7/signal.7.html

#### Determine how coredumps are saved

The coredump filename and location can be configured with the sysctl knob
[`kernel.core_pattern`][pattern]. Often the core is generated in the current
working directory, or in `/tmp` with the name `core`, potentially suffixed with
the PID and/or process name. On some distributions coredumps are managed by
systemd and must be extracted from an opaque storage via the
[`coredumpctl`][coredumpctl] helper.

[pattern]: https://man7.org/linux/man-pages/man5/core.5.html
[coredumpctl]: https://www.freedesktop.org/software/systemd/man/coredumpctl.html

To determine how coredumps are saved, you can run:

```bash
sudo sysctl kernel.core_pattern
```

#### Adjusting the `ulimit`

Normally the coredump generation is disabled via `ulimit`, and needs to be
adjusted first. To do so, in the same terminal that you'll later run the
application that you want to create a test case for, run:

```bash
ulimit -c unlimited
```

#### Creating the coredump

Via the executable name:

```bash
pkill -ILL <target application name>
```

Via the PID:

```bash
kill -ILL <pid>
```

After running one the above commands, if everything went well, you should see
a line containing `(core dumped)` in the stdout of the target application.

### Option 2: via GDB

This variant is particularly interesting because it allows you to single-step
to a very particular state that you want test coverage for and then create a
coredump. To do so, simply use gdb as usual and then type `gcore` once the
application is in the desired state. The path of the created coredump will be
printed on stdout.

The `gcore` command is also available as a standalone binary that can be
invoked directly from a shell (outside GDB) by typing:

```bash
gcore $PID
```

### Option 3: from within BPF

In some cases it's hard to use GDB to catch the application in a particular
state because it occurs very rarely. If the condition that you want to test
can be detected by a particular condition being true in the unwinder code,
you can use the [`DEBUG_CAPTURE_COREDUMP()` macro][macro] to kill and coredump 
the process that triggered it. You'll have to prepare your environment in the 
same manner as described in the ["Option 1"][opt1] section.

[opt1]: #option-1-using-coredump-new
[macro]: https://github.com/open-telemetry/opentelemetry-ebpf-profiler/blob/319d980b2406f40e68e850f429c38e28aed69e36/support/ebpf/bpfdefs.h#L116

## Extracting coredumps or modules

The actual coredumps are stored in an opaque storage solution and identified
within the test cases JSON file by their unique ID. The ID is stored in the 
`coredump-ref` field for the coredump file itself and in the `ref` field for 
the modules bundled with the test case (`modules` array).

In order to retrieve a coredump or a module, simply find the associated ID in 
the JSON file, then run:

```bash
./coredump export-module -id <ID from JSON> -out path/to/write/file/to
```

## Debugging the BPF code

To debug a failing test case it is advisable to build the tests as follows:

```bash
CGO_CFLAGS='-O0 -g' go test -c -gcflags="all=-N -l"
```

This will build the tests as a standalone binary and disable all optimizations 
which allows for a smooth single-stepping experience in both `gdb` and `dlv`.

You can now debug the BPF C code by running a specific test case in GDB:

```bash
gdb --args ./coredump.test -test.v -test.run \
  TestCoreDumps/testdata/arm64/java.PrologueEpilogue.epi.add-sp-sp.377026.json
```

A breakpoint on `native_tracer_entry` tends to be a good entry-point for
single-stepping.

## Cleaning up the coredump storage

The `coredump` helper provides a subcommand for cleaning both the local and
the remote storage:

```bash
./coredump clean
```

This will remove any data that is not referenced by any test case. The 
subcommand defaults to only cleaning the local storage. To also clean the 
remote data, pass the `-remote` argument:

```bash
./coredump clean -remote
```

The remote deletion defaults to only deleting data that has been uploaded more 
than 6 months ago. This ensures that you don't accidentally delete data for new
tests that have been proposed on a different branch that your current branch
isn't aware of, yet.

To see what will be deleted before actually committing to it, you can pass the 
`-dry-run` argument:

```bash
./coredump clean -remote -dry-run
```

## Updating all test cases

If a change in the unwinding causes many tests to produce different output,
you can use the `./coredump rebase` command to re-generate the thread array
for each test case based on current unwinding.

## Updating the tests to support new BPF maps

Please note that if your new feature adds new BPF maps then you will need to 
add references to this map manually to this package. This is because we do not 
currently support adding maps in an  automated fashion. The best way to do this
is to look through existing code in this package and to see where existing code 
refers to particular BPF maps.
