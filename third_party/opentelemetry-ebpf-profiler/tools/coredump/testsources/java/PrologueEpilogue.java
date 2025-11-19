/*
gdb -x ./javagdbinit --args java -Xcomp \
    -XX:+UnlockDiagnosticVMOptions \
    "-XX:CompileCommand=dontinline *PrologueEpilogue.*" \
    "-XX:CompileCommand=BreakAtExecute *PrologueEpilogue.*" \
    "-XX:CompileCommand=compileonly *PrologueEpilogue.*" \
    -XX:+PrintOptoAssembly -XX:+PrintAssembly -XX:-UseOnStackReplacement \
    -XX:-TieredCompilation PrologueEpilogue
*/

class PrologueEpilogue {
    static int a() {
        return b();
    }

    static int b() {
        return c();
    }

    static int c() {
        return 42;
    }

    public static void main(String [] argv) {
        int ctr = 123;
        for (long i = 0; i < 100; ++i) {
            ctr += a();
        }
        System.out.println(ctr);
    }
}
