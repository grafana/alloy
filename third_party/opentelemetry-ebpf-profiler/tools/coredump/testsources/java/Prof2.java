// Triggers "vtable chunks" frames

import java.util.function.Supplier;

public class Prof2 {

    public static void main(String[] args) {
        Supplier[] suppliers = {
                () -> 0,
                () -> 1.0,
                () -> "abc",
                () -> true
        };

        for (int i = 0; i >= 0; i++) {
            suppliers[i % suppliers.length].get();
        }
    }
}
