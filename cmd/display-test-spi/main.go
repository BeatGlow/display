package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/beatglow/display/conn"
)

func main() {
	busFlag := flag.Int("bus", 0, "SPI bus")
	deviceFlag := flag.Int("device", 0, "SPI device")
	flag.Parse()

	c, err := conn.OpenSPI(*busFlag, *deviceFlag)
	if err != nil {
		log.Fatalln("open failed: ", err)
	}
	fmt.Println("connected using", c)
	if err = c.Close(); err != nil {
		log.Fatalln("close failed: ", err)
	}
}
