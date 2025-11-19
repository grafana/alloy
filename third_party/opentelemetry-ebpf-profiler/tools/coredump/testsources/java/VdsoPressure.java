import java.lang.System;

class VdsoPressure {
    public static void main(String [] argv) {
        long ctr = 0;
        for (long i = 0; i < 10_000_000_000L; ++i) {
            ctr += System.nanoTime();
        }
        System.out.println(ctr);
    }
}

