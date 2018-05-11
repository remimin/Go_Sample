package main

import (
	"strings"
	"fmt"	
	"os"
	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

/*
func FindVMByName(conn * libvirt.Connect, name string){
	if err != nil {
		fmt. Println("Connect error")
	}
	dom, _ := conn.LookupDomainByName(name)
	if dom == nil {
		fmt.Println("dom")
		return
	}
	 fmt.Println("Find vm done.")
}

*/
func CreateVM(conn * libvirt.Connect){
	var Mem uint
	Mem = 524288
	Lmem :=  &libvirtxml.DomainMemory{Value: Mem}
	Lcpu :=	&libvirtxml.DomainVCPU{Placement: "static", Value: 1}
	domcfg := &libvirtxml.Domain{Type: "kvm", Name: "go-test",
		Memory: Lmem, VCPU:Lcpu}
	xmldoc, _:= domcfg.Marshal()
	fmt.Println(xmldoc)
			
}

type Device struct {
	UUID		string
	Type 		string
	Size		uint64
	IsBoot		bool
	MountPoint	string
}

type Instance struct {
	UUID	    string
	Name        string
	ImageName   string
	Cpu         int
	Mem         int
	DeviceMap   []Device
	Host        string
	Description string
	OSType		string
	Arch		string
}

func XML(vm * Instance, caps * libvirtxml.Caps){
	guest := new(libvirtxml.Domain)
	guest.Type = "kvm"
	guest.Name = "go-test"
	guest.Memory = &libvirtxml.DomainMemory{Value: uint(vm.Mem * 1024)}//From Mib to KiB
	guest.VCPU = &libvirtxml.DomainVCPU{Value: vm.Cpu, Placement: "static"}
	
	guest.Description = vm.Description
	guest.Name = vm.Name
	guest.UUID = vm.UUID
	guest.OS = &libvirtxml.DomainOS{}
	guest.OS.Type = &libvirtxml.DomainOSType{Type: "hvm"}
	guest.OS.BootDevices = make([]libvirtxml.DomainBootDevice, 1)
	guest.OS.BootDevices[0] = libvirtxml.DomainBootDevice{Dev: "hd"}
	guest.SysInfo = &libvirtxml.DomainSysInfo{Type: "smbios", }
	guest.OS.SMBios = &libvirtxml.DomainSMBios{Mode: "sysinfo"}
	guest.Features = &libvirtxml.DomainFeatureList{ACPI: &libvirtxml.DomainFeature{},
					APIC: &libvirtxml.DomainFeatureAPIC{}}
	guest.CPUTune = &libvirtxml.DomainCPUTune{
		Shares: &libvirtxml.DomainCPUTuneShares{Value: uint(1024 * vm.Cpu)}}
	guest.Clock = new(libvirtxml.DomainClock)
	switch vm.OSType {
	case "windows":
		guest.Clock.Offset = "localtime"
	case "linux":
		guest.Clock.Offset = "utc"
	default:
		guest.Clock.Offset = "utc"
	}

	setClockTimer(vm, guest.Clock)

	guest.Devices = &libvirtxml.DomainDeviceList{Disks: make([]libvirtxml.DomainDisk, 0)}
	setGuestStorageDevices(vm, guest)
	src_br := &libvirtxml.DomainInterfaceSourceBridge{Bridge: "vmbridge"}
	tgt_dev := &libvirtxml.DomainInterfaceTarget{Dev: "vnet3"}
	mod := &libvirtxml.DomainInterfaceModel{Type: "virtio"}
	vif := libvirtxml.DomainInterface{
		Source: &libvirtxml.DomainInterfaceSource{Bridge:src_br},
		Target: tgt_dev,
		Model: mod,
	}
	guest.Devices.Interfaces = []libvirtxml.DomainInterface{vif}
	xmldoc, _:= guest.Marshal()
        fmt.Println(xmldoc)
}

func setClockTimer(vm * Instance, clock * libvirtxml.DomainClock) {
	clock.Timer = make([]libvirtxml.DomainTimer, 0)

	// Tpit := libvirtxml.DomainTimer{Name: "pit", TickPolicy: "delay"}
	clock.Timer = append(clock.Timer,
		libvirtxml.DomainTimer{Name: "pit", TickPolicy: "delay"})
	//Trtc := libvirtxml.DomainTimer{Name: "rtc", TickPolicy: "catchup"}
	clock.Timer = append(clock.Timer,
		libvirtxml.DomainTimer{Name: "rtc", TickPolicy: "catchup"})
	switch vm.Arch {
	case "X86_64", "I686", "":
		// Thpet := libvirtxml.DomainTimer{Name: "hpet", Present: "no"}
		clock.Timer = append(clock.Timer,
			libvirtxml.DomainTimer{Name: "hpet", Present: "no"})
	}
	if vm.OSType == "windows" {
		// Thyperv := libvirtxml.DomainTimer{Name:"hypervclock", Present: "yes"}
		clock.Timer = append(clock.Timer,
			libvirtxml.DomainTimer{Name: "hpet", Present: "no"})
	}
}


func setGuestStorageDevices(vm * Instance, guest * libvirtxml.Domain) error{
	for i:=0; i < len(vm.DeviceMap); i++ {
		disk := &libvirtxml.DomainDisk{}
		switch vm.DeviceMap[i].Type {
		case "rbd", "ceph":
			err := setRbdDisk(&vm.DeviceMap[i], vm, disk)
			if err != nil {
				return err
			}
		}

		disk.Target = &libvirtxml.DomainDiskTarget{Dev: vm.DeviceMap[i].MountPoint,
			Bus: "virtio"}
		guest.Devices.Disks = append(guest.Devices.Disks, *disk)
	}
	return nil
}

func setRbdDisk(disk * Device, vm * Instance, dom_disk * libvirtxml.DomainDisk) error {
	mon_host := "host1, host2, host3"
	hosts := make([]libvirtxml.DomainDiskSourceHost, 0)
	for _, h := range strings.Split(mon_host, ",") {
		hosts = append(hosts, libvirtxml.DomainDiskSourceHost{Name: h, Port: "6789"})
	}

	// Config ceph Auth from config
	auth := libvirtxml.DomainDiskAuth{Username: "admin",
		Secret: &libvirtxml.DomainDiskSecret{Type: "ceph", Usage: "cephadmin"}}
	rbd_vol := libvirtxml.DomainDiskSourceNetwork{Protocol: "rbd", Name: "rbd_vol",
		Hosts: hosts, Auth: &auth}
	source := libvirtxml.DomainDiskSource{Network: &rbd_vol}
	dom_disk.Device = "disk"
	dom_disk.Driver = &libvirtxml.DomainDiskDriver{Type: "raw", Cache: "writeback"}
	dom_disk.Source = &source
	dom_disk.Auth = &auth
	return nil
}

func PrintCaps(conn * libvirt.Connect){
	caps, _ := conn.GetCapabilities()
	fmt.Println(caps)
}

func GetDetails(conn * libvirt.Connect){
	hostname, _ := conn.GetHostname()
	virt_type, _ := conn.GetType()
	version, _ := conn.GetVersion()
	fmt.Println("hostname:", hostname, "type:", virt_type,
		"version:", version)
}

func main (){
	if len(os.Args) < 2 {
		 os.Exit(1)
	}
	args := os.Args[1]
	fmt.Println("Args: ", args)
	conn, _:= libvirt.NewConnect("qemu:///system")
	switch args {
		case "details":
			GetDetails(conn)
		case "caps":
			PrintCaps(conn)
		case "create":
			CreateVM(conn)
		case "test":
			caps, _ := conn.GetCapabilities()
			vm := Instance{UUID: "8ccf4be1-c026-4dec-b834-03e2bf95694d",
					Name: "VMtest",
					ImageName: "image",
					Cpu: 1,
					Mem: 1,
					DeviceMap: []Device{{Type: "rbd"}},
					Host: "local",
					Description: "TestXML"}
			Host := new(libvirtxml.Caps)
			Host.Unmarshal(caps)
			XML(&vm, Host)
	}
	// Host := new(libvirtxml.Caps)
	// Host.Unmarshal(caps)
	// fmt.Println("caps:", Host.Host.UUID)
}
