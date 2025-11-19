import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.lang.Thread;

public class Lambda1 {
    public static Comparator<Double> comparator1() {
        return (d1, d2) -> {
            try {
               Thread.sleep(10000); // generate core from this location
            } catch (Exception e) {
            }
            return d1.compareTo(d2);
        };
    }

    public static void main(String[] args) {
        ArrayList list = new ArrayList<>();
        list.add(1.9);
        list.add(1.2);
        Collections.sort(list, comparator1());
    }
}
