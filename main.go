package main

import "fmt"

func main() {

	tchan := make(chan int)
	close(tchan)

	r := <-tchan
	fmt.Println(r)

}
