# Building a Backtrace for Python Code

Python code objects do not directly store the line number they are associated with. There
is a field `f_lineno` inside a `PyFrameObject`, but this only stores the line number if
tracing is activated. Instead, we can retrieve the bytecode offset of the current
instruction from the `f_lasti` field of the frame object. The code object then contains a
field called `co_lnotab`, which allows one to map from bytecode offsets to line numbers.
The process of mapping bytecode offsets to line numbers is described in
[this][lnotab-notes] file. Note that this representation may change between Python
versions. The code in this repository is written as of Python 3.7.1. 

In the interpreter, the `PyCode_Addr2Line` function provides an implementation of this
mapping. A python implementation can be found in Python's gdb [helpers][libpython].
Further information on code objects can be found [here][code objects].

## Difficulties in Determining the Correct Line Number

A potential edge case on mapping offsets to line numbers is mentioned in the
[notes][lnotab-notes] on tracing. It seems that if a function has an implicit return then
the offset for the opcodes associated with that return may map to a line number for Python
code that is not actually being executed. For example, in the notes this function is
presented (see the notes for the bytecode): 

```python
1: def f(a):
2:    while a:
3:       print(1)
4:       break
5:    else:
6:       print(2)
```

The opcode for returning from the function is generated and associated with line 6. If we
reach line 4 we then jump to the return opcode. At this point, if we attempt to map the
opcode to a line number we will incorrectly deduce that we are at line 6. Similarly, a
single opcode will be generated to break from the loop. It will be jumped to if `a` is
false on line 2, or by reaching line 4. If, say, `a` is false we jump to the `BREAK_LOOP`
opcode, but at this point if we try to determine the line number we will incorrectly
deduce that we are at line 4, not 2. 

I am not sure how to deal with such situations yet. It is possible that we can't, and
just have to accept that in these limited cases we may assign a frame to the wrong line 
number. 

## Mapping Bytecode Offsets to Line Numbers in Userspace

There appears to be no upper limit on the number of lines of code that a code object may
represent. Calculating line number information therefore requires a loop who's number of
iterations has a very large upper bound. In eBPF we can use tail-calls to get around the
size limit of 4906 instructions per program, but there is still an upper limit of 32 such
calls. 

Instead of trying to map the bytecode offset to a line number in the eBPF program it is
likely better to do it in userspace. The .pyc files associated with each Python source
file contain `PyCodeObject` instances, that in turn contain the `co_lnotab` tables that
we require to perform the mapping from bytecode offsets to line numbers. The `dis` module
provides functions for processing compiled Python code, so if we log the bytecode offsets
in kernel space we may be able to use functions from `dis` to recover the line number
information.

Given a .pyc file we can construct a code object using `dis`. Code objects can be nested,
so the code object for a module may contain code objects for classes, which in turn may
contain code objects for functions and so on. Thus we need the eBPF side of things to log
some piece of information that uniquely identifies the code object associated with the
bytecode offset it is logging. The `co_firstlineno` field of a `PyCodeObject` seems like a
good candidate for this. It provides the first source line number with which the code
object is associated. Unfortunately, it is not unique. For example, in the following code
there may be two code objects created, both that have a `co_firstlineno` value of 1. 

```python
def foo():
    for x in range(100):
       pass
```

The first code object will look as follows. It creates the function and associates it
with the correct name.

```
1 0 LOAD_CONST               0 (<code object foo at 0x7f3206e33ed0, file "test.py", line 1>)
  2 LOAD_CONST               1 ('foo')
  4 MAKE_FUNCTION            0
  6 STORE_NAME               0 (foo)
```

Another code object will then be created to provide the actual code of the function that
is referenced at the first opcode.

To get around the above issue, we identify code objects using a hash of several fields.
For the exact fields, see the Python tracer. At the moment the are `co_firstlineno`,
`co_argcount`, `co_kwonlyargcount` and `co_flags`.

Once we have found the correct code object, we can then generate a list of `(offset,
lineno)` pairs using the `dis.findlinestarts(code)` function. With this we can then
convert offsets to line numbers. In our real version we can build a multi-level map ahead
of time, to enable converting a `(filename, co_firstlineno, bytecode offset)` triple to a
line number.

[lnotab-notes]: https://github.com/python/cpython/blob/37788bc23f6f1ed0362b9b3b248daf296c024849/Objects/lnotab_notes.txt
[libpython]: https://github.com/python/cpython/blob/37788bc23f6f1ed0362b9b3b248daf296c024849/Tools/gdb/libpython.py#L642
[code objects]: https://leanpub.com/insidethepythonvirtualmachine/read#leanpub-auto-code-objects
