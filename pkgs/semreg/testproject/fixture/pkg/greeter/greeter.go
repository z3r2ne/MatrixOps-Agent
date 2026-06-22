package greeter

import "fmt"

func Greet(name string) string {
	return fmt.Sprintf("Hello, %s! This is the built-in semreg test project.", name)
}
