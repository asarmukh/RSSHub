package main

import (
	"fmt"
	"log"
	"os"

	"rsshub/internal/cmd"
	"rsshub/internal/helper"

	_ "github.com/lib/pq"
)

func main() {
	if len(os.Args) < 2 {
		helper.PrintHelp()
		os.Exit(1)
	}

	cmdName := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmdName {
	case "--help", "-h", "help":
		helper.PrintHelp()
	case "fetch":
		err = cmd.Fetch(args)
	case "add":
		err = cmd.Add(args)
	case "list":
		err = cmd.List(args)
	case "delete":
		err = cmd.Delete(args)
	case "articles":
		err = cmd.Articles(args)
	case "set-interval":
		err = cmd.SetInterval(args)
	case "set-workers":
		err = cmd.SetWorkers(args)
	default:
		fmt.Printf("unknown command: %s\n\n", cmdName)
		helper.PrintHelp()
		os.Exit(1)
	}

	if err != nil {
		log.Printf("%v", err)
		os.Exit(1)
	}
}
