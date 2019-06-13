// main.go

package main

func main() {
	a := App{}
	a.Initialize("api", "api", "api")

	a.Run(":8080")
}
