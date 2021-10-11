package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/go-ping/ping"
	"github.com/jessevdk/go-flags"
	"github.com/xuri/excelize/v2"
)

type Options struct {
	Host  string `short:"h" long:"host" description:"The host to ping" optional:"true" default:"www.google.com"`
	Excel string `short:"x" long:"excel" description:"The name of the excel file to write to" optional:"true" default:"output.xlsx"`
	Size  int    `short:"s" long:"size" description:"The size of the ICMP packet" optional:"true" default:"64"`
}

func main() {
	options := &Options{}

	if _, err := flags.Parse(options); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	excel := excelize.NewFile()
	excel.SetCellValue("Sheet1", "A1", "Sequence")
	excel.SetCellValue("Sheet1", "B1", "Duration")
	excel.SetCellValue("Sheet1", "C1", "Bytes")
	excel.SetCellValue("Sheet1", "D1", "IP Address")

	pinger, err := ping.NewPinger(options.Host)
	if err != nil {
		panic(err)
	}

	// listen for Ctrl-C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			pinger.Stop()
		}
	}()

	pinger.Size = options.Size - 8 // if 64 then 56

	counter := 2
	lastSequence := 0
	red := color.New(color.FgRed, color.Bold)
	pinger.OnRecv = func(pkt *ping.Packet) {
		if (pkt.Seq - lastSequence) > 1 {
			red.Printf("WARN: packets lost from sequence no. %d to %d\n", lastSequence, pkt.Seq)
		}
		fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v\n", pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
		lastSequence = pkt.Seq
		excel.SetCellValue("Sheet1", fmt.Sprintf("A%d", counter), pkt.Seq)
		if rtt, err := strconv.ParseFloat(strings.ReplaceAll(pkt.Rtt.String(), "ms", ""), 64); err == nil {
			excel.SetCellFloat("Sheet1", fmt.Sprintf("B%d", counter), rtt, 4, 64)
		} else {
			excel.SetCellValue("Sheet1", fmt.Sprintf("A%d", counter), strings.ReplaceAll(pkt.Rtt.String(), "ms", ""))
		}
		excel.SetCellInt("Sheet1", fmt.Sprintf("C%d", counter), pkt.Nbytes)
		excel.SetCellValue("Sheet1", fmt.Sprintf("D%d", counter), pkt.IPAddr.String())
		counter++
	}

	pinger.OnDuplicateRecv = func(pkt *ping.Packet) {
		fmt.Printf("%d bytes from %s: icmp_seq=%d time=%v ttl=%v (DUP!)\n", pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt, pkt.Ttl)
	}

	pinger.OnFinish = func(stats *ping.Statistics) {
		fmt.Printf("\n--- %s ping statistics ---\n", stats.Addr)
		fmt.Printf("%d packets transmitted, %d packets received, %v%% packet loss\n", stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n", stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
		if err := excel.SaveAs(options.Excel); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("PING %s (%s):\n", pinger.Addr(), pinger.IPAddr())
	err = pinger.Run()
	if err != nil {
		fmt.Printf("error: %v\n", err)
	}
}
