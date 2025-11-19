// Reduced variant of Jonas Kunz' CPU burner application. Pressures SHA256
// which is implemented via StubRoutines.

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Random;

public class ShaShenanigans {
    public static volatile Object sink;

    public static void main(String[] args) {
        while (true) {
            shaShenanigans();
        }
    }

    public static void shaShenanigans() {
        long start = System.nanoTime();
        while ((System.nanoTime() - start) < 100_000_000L) {
            sink = hashRandomStuff();
        }
    }

    private static byte[] hashRandomStuff() {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            Random rnd = new Random();
            byte[] buffer = new byte[1024];
            rnd.nextBytes(buffer);
            for(int i=0; i<5000; i++) {
                digest.update(buffer);
            }
            return digest.digest();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }
}
