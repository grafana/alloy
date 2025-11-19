function bar(a, s) {
	a(s)
	//ads
}

function foo() {
	bar(a, "Hello world!")
}

function doit() {
	for (i = 0; i < 1000; i++) {
		foo()
	}
}

console.trace("I am here");
a = console.log
for (j = 0; j < 1000; j++) {
	doit()
}

