package main

import "fmt"

func main() {
	fmt.Print(((20000 - 10000) - ((20000 - 100000) * 7 / 10)) * 100000)
}

func calculateInitialPrice(buySlopeNum, buySlopeDen, initialGoal, totalSupply, initialReserve int) int {
	return (buySlopeNum * initialGoal / buySlopeDen) * (initialGoal - totalSupply + initialReserve)
}
