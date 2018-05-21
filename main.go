package main

import (
	"os"
	"fmt"
	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
	"github.com/remimin/Go_Sample/vm_mgr"
	"io/ioutil"
)

func main () {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	args := os.Args[1]
	fmt.Println("Args: ", args)
	conn, _:= libvirt.NewConnect("qemu:///system")
	switch args {
	case "details":
		vm_mgr.GetDetails(conn)
	case "caps":
		vm_mgr.PrintCaps(conn)
	case "create":
		vm_mgr.CreateVM(conn)
	case "test":
		caps, _ := conn.GetCapabilities()
		vm := vm_mgr.Instance{UUID: "8ccf4be1-c026-4dec-b834-03e2bf95694d",
			Name: "VMtest",
			ImageName: "image",
			Cpu: 1,
			Mem: 1,
			DeviceMap: []vm_mgr.Device{{Type: "rbd"}},
			Host: "local",
			Description: "TestXML"}
		Host := new(libvirtxml.Caps)
		Host.Unmarshal(caps)
		vm_mgr.XML(&vm, Host)
	case "guest":
		root_disk := vm_mgr.GuestDisk{Type: "rbd", UUID: "8ccf4be1-c026-4dec-b834-03e2bf95694e",
			Source: "kolla_pool_1/volume-ff99fb47-9b78-4fcf-98e1-c4f60bdb77cb"}
		guest := vm_mgr.GuestConf{UUID: "8ccf4be1-c026-4dec-b834-03e2bf95694d",
			Name: "VMGuestTest",
			ImageName: "image",
			Cpu: 1,
			Mem: 1,
			DiskList: []vm_mgr.GuestDisk{root_disk},
			Host: "local",
			Description: "TestXML"}
		xml, err := guest.GetGuestXML(conn)
		if err != nil {
			fmt.Println("XML Generation from guestconf failed.")
		}
		fmt.Println(xml)
		err = ioutil.WriteFile("/tmp/guest.xml", []byte(xml), 0644)
		if err != nil {
			fmt.Println("Write xml guest failed.")
		}
	}
}
