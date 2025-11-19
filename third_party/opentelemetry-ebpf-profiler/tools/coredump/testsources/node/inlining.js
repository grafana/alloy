
function add(a, b) {
	console.trace("here")
	return a + b
}

function add3(a, b, c) {
	return add(a, add(b, c))
}

function test(a, b, c, d) {
	return add3(a, b, c) == d
}

function submain() {
	for (var i = 0; i < 1000; i++) {
		test(i, 2*i, 3*i, 100)
	}
}

function main() {
	for (var i = 0; i < 1000; i++) {
		submain()
	}
}

main()
