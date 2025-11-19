function bar(a, s) {
	a(s)
	//ads
}

function foo() {
	bar(a, "Hello world!")
}

console.trace("I am here");
a = console.log
for (i = 0; i < 10000000; i++) {
	foo()
}

