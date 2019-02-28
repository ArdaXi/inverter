package main

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

func main() {
	iface := "switch0"
	filter := "tcp and host 192.168.179.37 and greater 161"
	if handle, err := pcap.OpenLive(iface, 1600, true, pcap.BlockForever); err != nil {
		panic(err)
	} else if err := handle.SetBPFFilter(filter); err != nil { // optional
		panic(err)
	} else {
		fmt.Printf("Listening on %s with filter %s.\n", iface, filter)
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for packet := range packetSource.Packets() {
			fmt.Println("Got packet")
			if data := packet.ApplicationLayer(); data != nil {
				fmt.Printf("%s\n", data)
			}
		}
	}
}
