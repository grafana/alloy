// Works together with DeoptFoo, and triggers deoptimized frames to be seen on the stack

class Deopt {
	public int foo;

	public void Bar() {
		foo = foo * 123;
	}

	public void Handle(int x) {
		for (int i = 0; i < 1000000000; i++) {
			Bar();
		}
		if (x < 10) {
			Handle(x + 1);
		} else {
			try {
				ClassLoader.getSystemClassLoader().loadClass("DeoptFoo");
				while (true) {
					System.out.print("foo\n");
				}
			} catch (Exception e) {
			}
		}
	}

	public static void main( String []args ) throws InterruptedException {
		Deopt foo = new Deopt();
		foo.foo = 2;
		while (true) {
			foo.Handle(1);
		}
	}
}
