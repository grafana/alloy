
public class Prof1 {

    public static void main(String[] args) {
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < 1000000; i++) {
            sb.append("ab");
            sb.delete(0, 1);
        }
        System.out.println(sb.length());
    }
}
