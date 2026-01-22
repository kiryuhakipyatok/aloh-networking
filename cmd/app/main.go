package main

import (
	"networking/internal/app"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../../.env"); err != nil {
		panic(err)
	}
	app.Run()
}
