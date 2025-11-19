package co.elastic.profiling;

import java.security.SecureRandom;

public class HelloGraal {

    public static void main(String[] args) {
        SecureRandom rand = new SecureRandom();
        long maxCounter = 10000000l;

        for (long i = 0; i < maxCounter; i++) {
            // trigger page fault
            byte[] randomBytes = new byte[4444];
            rand.nextBytes(randomBytes);

            if (i > 0 && (maxCounter - i) % 10000 == 0) {
                System.out.printf("Progress reading from rand: %d out of %d %n", i , maxCounter);
            }
        }

        System.out.println("Done!");
    }
}
