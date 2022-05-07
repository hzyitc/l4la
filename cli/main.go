package main

import (
	"os"
	"strconv"
)

var version = "v0.0.0"

func usage() {
	print(os.Args[0] + " (" + version + ")\n")
	print("Usage:\n")
	print("  " + os.Args[0] + " server <listen port> <service address>\n")
	print("  " + os.Args[0] + " client <listen port> <server address> <connection number>\n")
}

func main() {
	args := os.Args[1:]

	if len(args) < 3 || len(args) > 4 {
		usage()
		return
	}

	switch args[0] {
	case "server":
		port, err := strconv.Atoi(args[1])
		if err != nil {
			usage()
			return
		}
		NewServer(port, args[2])
	case "client":
		port, err := strconv.Atoi(args[1])
		if err != nil {
			usage()
			return
		}

		conn_number, err := strconv.Atoi(args[3])
		if err != nil {
			usage()
			return
		}

		NewClient(port, args[2], conn_number)
	default:
		usage()
	}
}
