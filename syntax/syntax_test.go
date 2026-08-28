package syntax_test

import (
	"fmt"
	"os"

	"github.com/grafana/alloy/syntax"
)

func ExampleUnmarshal() {
	// Character is our block type which holds an individual character from a
	// book.
	type Character struct {
		// Name of the character. The name is decoded from the block label.
		Name string `alloy:",label"`
		// Age of the character. The age is a required attribute within the block,
		// and must be set in the config.
		Age int `alloy:"age,attr"`
		// Location the character lives in. The location is an optional attribute
		// within the block. Optional attributes do not have to bet set.
		Location string `alloy:"location,attr,optional"`
	}

	// Book is our overall type where we decode the overall Alloy file into.
	type Book struct {
		// Title of the book (required attribute).
		Title string `alloy:"title,attr"`
		// List of characters. Each character is a labeled block. The optional tag
		// means that it is valid not provide a character block. Decoding into a
		// slice permits there to be multiple specified character blocks.
		Characters []*Character `alloy:"character,block,optional"`
	}

	// Create our book with two characters.
	input := `
		title = "Wheel of Time"

		character "Rand" {
			age      = 19
			location = "Two Rivers"
		}

		character "Perrin" {
			age      = 19
			location = "Two Rivers"
		}
	`

	// Unmarshal the config into our Book type and print out the data.
	var b Book
	if err := syntax.Unmarshal([]byte(input), &b); err != nil {
		panic(err)
	}

	fmt.Printf("%s characters:\n", b.Title)

	for _, c := range b.Characters {
		if c.Location != "" {
			fmt.Printf("\t%s (age %d, location %s)\n", c.Name, c.Age, c.Location)
		} else {
			fmt.Printf("\t%s (age %d)\n", c.Name, c.Age)
		}
	}

	// Output:
	// Wheel of Time characters:
	// 	Rand (age 19, location Two Rivers)
	// 	Perrin (age 19, location Two Rivers)
}

// This example shows how functions may be called within user configurations.
// We focus on the `env` function from the standard library, which retrieves a
// value from an environment variable.
func ExampleUnmarshal_functions() {
	// Set an environment variable to use in the test.
	_ = os.Setenv("EXAMPLE", "Jane Doe")

	type Data struct {
		String string `alloy:"string,attr"`
	}

	input := `
		string = sys.env("EXAMPLE")
	`

	var d Data
	if err := syntax.Unmarshal([]byte(input), &d); err != nil {
		panic(err)
	}

	fmt.Println(d.String)
	// Output: Jane Doe
}

func ExampleUnmarshalValue() {
	input := `3 + 5`

	var num int
	if err := syntax.UnmarshalValue([]byte(input), &num); err != nil {
		panic(err)
	}

	fmt.Println(num)
	// Output: 8
}

func ExampleMarshal() {
	type Person struct {
		Name     string `alloy:"name,attr"`
		Age      int    `alloy:"age,attr"`
		Location string `alloy:"location,attr,optional"`
	}

	p := Person{
		Name: "John Doe",
		Age:  43,
	}

	bb, err := syntax.Marshal(p)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(bb))
	// Output:
	// name = "John Doe"
	// age  = 43
}

func ExampleMarshalValue() {
	type Person struct {
		Name string `alloy:"name,attr"`
		Age  int    `alloy:"age,attr"`
	}

	p := Person{
		Name: "John Doe",
		Age:  43,
	}

	bb, err := syntax.MarshalValue(p)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(bb))
	// Output:
	// {
	// 	name = "John Doe",
	// 	age  = 43,
	// }
}
