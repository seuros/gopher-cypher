package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/seuros/gopher-cypher/src/driver"
)

func pingCommand(args []string) error {
	fs := flag.NewFlagSet("ping", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	urlFlag := fs.String("url", os.Getenv("CYQ_URL"), "Connection URL (or set CYQ_URL)")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return &exitError{code: 0}
		}
		return usageErrorf(2, "%v", err)
	}

	if *urlFlag == "" {
		return usageErrorf(2, "Missing --url (or set CYQ_URL)")
	}

	dr, err := driver.NewDriver(*urlFlag)
	if err != nil {
		return err
	}
	defer func() { _ = dr.Close() }()

	fmt.Println("OK")
	return nil
}
