package vm_mgr

import (
	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
	"strings"
	"fmt"
)

const (
	X86_64 = "x86_64"
	I686 = "i686"
)

/* cpu mode
** host-model: Default cpu mode.
** host-passthrough:
** custom: Customize cpu mode, cpu_model is required.

*/
const (
	HOST_MODEL = "host-model"
	CUSTOM = "custom"
	HOST_PT = "host-passthrough"
	NONE = "none"
)

// cpu model



type GuestDisk struct {
	UUID		string
	Type 		string
	IsBoot		bool
	MountPoint	string
	Source 		string
}

type GuestConf struct {
	UUID        string
	Name        string
	ImageName	string
	Cpu         int
	Mem         uint
	DiskList   	[]GuestDisk
	Host        string
	Description string
	OSType		string
	Arch		string
	QGA			bool
}

func (guestconf * GuestConf)GetGuestXML(conn * libvirt.Connect) (string, error) {
	guest := new(libvirtxml.Domain)
	guest.Type = "kvm"
	guest.Memory = &libvirtxml.DomainMemory{Value: uint(guestconf.Mem * 1024)}//From Mib to KiB
	guest.VCPU = &libvirtxml.DomainVCPU{Value: guestconf.Cpu, Placement: "static"}
	guest.Description = guestconf.Description
	guest.Name = guestconf.Name
	guest.UUID = guestconf.UUID
	guest.OS = &libvirtxml.DomainOS{}
	guest.OS.Type = &libvirtxml.DomainOSType{Type: "hvm"}
	guest.OS.BootDevices = make([]libvirtxml.DomainBootDevice, 1)
	guest.OS.BootDevices[0] = libvirtxml.DomainBootDevice{Dev: "hd"}

	caps, err := conn.GetCapabilities()
	if err != nil {

		return "", nil
	}
	host_caps :=  new(libvirtxml.Caps)
	host_caps.Unmarshal(caps)

	// Set guest virt type. TODO support UEFI
	guest.SysInfo = &libvirtxml.DomainSysInfo{Type: "smbios"}
	guestconf.setGuestSysInfo(guest, host_caps)
	guest.OS.SMBios = &libvirtxml.DomainSMBios{Mode: "sysinfo"}
	// Set Feature
	guest.Features = &libvirtxml.DomainFeatureList{ACPI: &libvirtxml.DomainFeature{},
		APIC: &libvirtxml.DomainFeatureAPIC{}}
	// Set Cputune
	guest.CPUTune = &libvirtxml.DomainCPUTune{
		Shares: &libvirtxml.DomainCPUTuneShares{Value: uint(1024 * guestconf.Cpu)}}
	// Set clock
	guest.Clock = new(libvirtxml.DomainClock)
	switch guestconf.OSType {
	case "windows":
		guest.Clock.Offset = "localtime"
	case "linux":
		guest.Clock.Offset = "utc"
	default:
		guest.Clock.Offset = "utc"
	}
	// TODO add virt type kvm & qemu
	guestconf.setClockTimer(guest.Clock)

	// Set devices
	guest.Devices = &libvirtxml.DomainDeviceList{Disks: make([]libvirtxml.DomainDisk, 0)}
	// Set storage devices
	err = guestconf.setGuestStorageDevices(guest)
	if err != nil {
		return "", err
	}
	// Set network interface
	guestconf.setDefaultInterface(guest)

	// Set Usb
	guestconf.setUsbConf(guest)
	// Set graphic vnc
	guestconf.setGraphics(guest)

	// set vedio driver
	guestconf.setDefaultVideo(guest)
	// Set MemBalloon
	guestconf.setMemBalloon(guest)

	if guestconf.QGA == true {
		guestconf.setQemuGuestAgent(guest)
	}

	return guest.Marshal()
}

func (guestconf * GuestConf)setCPU(clock * libvirtxml.DomainCPU) {
}

func (guestconf * GuestConf)setClockTimer(clock * libvirtxml.DomainClock) {
	clock.Timer = make([]libvirtxml.DomainTimer, 0)
	clock.Timer = append(clock.Timer,
		libvirtxml.DomainTimer{Name: "pit", TickPolicy: "delay"})
	clock.Timer = append(clock.Timer,
		libvirtxml.DomainTimer{Name: "rtc", TickPolicy: "catchup"})
	switch guestconf.Arch {
	case X86_64, I686, "":
		clock.Timer = append(clock.Timer,
			libvirtxml.DomainTimer{Name: "hpet", Present: "no"})
	}
	if guestconf.OSType == "windows" {
		clock.Timer = append(clock.Timer,
			libvirtxml.DomainTimer{Name: "hypervclock", Present: "yes"})
	}
}

func (guestconf * GuestConf)setGuestSysInfo(guest * libvirtxml.Domain, caps * libvirtxml.Caps){
	guest.SysInfo.System = &libvirtxml.DomainSysInfoSystem{Entry: make([]libvirtxml.DomainSysInfoEntry, 0)}
	guest.SysInfo.System.Entry = append(guest.SysInfo.System.Entry,
		libvirtxml.DomainSysInfoEntry{Name: "serial", Value: caps.Host.UUID})
	guest.SysInfo.System.Entry = append(guest.SysInfo.System.Entry,
		libvirtxml.DomainSysInfoEntry{Name: "uuid", Value: guestconf.UUID})
	guest.SysInfo.System.Entry = append(guest.SysInfo.System.Entry,
		libvirtxml.DomainSysInfoEntry{Name: "family", Value: "Virtual Machine"})
}

func (guestconf * GuestConf)setGuestStorageDevices(guest * libvirtxml.Domain) error{
	for i:=0; i < len(guestconf.DiskList); i++ {
		disk := &libvirtxml.DomainDisk{}
		switch guestconf.DiskList[i].Type {
		case "rbd", "ceph":
			err := guestconf.setRbdDisk(&guestconf.DiskList[i], disk)
			if err != nil {
				return err
			}
		case "local":
			guestconf.setLocalDisk(&guestconf.DiskList[i], guest.Devices)
		case "cdrom":
			guestconf.setCdrom(&guestconf.DiskList[i], guest.Devices)
		}
		disk.Target = &libvirtxml.DomainDiskTarget{Dev: guestconf.DiskList[i].MountPoint,
			Bus: "virtio"}
		disk.Serial = guestconf.DiskList[i].UUID
		guest.Devices.Disks = append(guest.Devices.Disks, *disk)
	}
	return nil
}

// Set rbd disk
func (guestconf * GuestConf)setRbdDisk(disk * GuestDisk, dom_disk * libvirtxml.DomainDisk) error {
	mon_host, err := GetMonHost()
	if err != nil {
		return err
	}
	hosts := make([]libvirtxml.DomainDiskSourceHost, 0)
	for _, h := range strings.Split(mon_host, ",") {
		mon := strings.Split(strings.Split(h, "/")[0], ":")
		hosts = append(hosts, libvirtxml.DomainDiskSourceHost{Name: mon[0], Port: mon[1]})
	}

	// Config ceph Auth from config
	auth := libvirtxml.DomainDiskAuth{Username: "admin",
		Secret: &libvirtxml.DomainDiskSecret{Type: "ceph", Usage: "cephadmin"}}
	rbd_vol := libvirtxml.DomainDiskSourceNetwork{Protocol: "rbd", Name: disk.Source,
		Hosts: hosts, Auth: &auth}
	source := libvirtxml.DomainDiskSource{Network: &rbd_vol}
	dom_disk.Device = "disk"
	dom_disk.Driver = &libvirtxml.DomainDiskDriver{Type: "raw", Cache: "writeback"}
	dom_disk.Source = &source
	dom_disk.Auth = &auth

	return nil
}

// TODO Set local disk
func (guestconf * GuestConf)setLocalDisk(disk * GuestDisk, guest * libvirtxml.DomainDeviceList){
}

// TODO Set local disk
func (guestconf * GuestConf)setCdrom(disk * GuestDisk, guest * libvirtxml.DomainDeviceList){
}

// TODO Guest NUMA set
func (guestconf * GuestConf)setGuestNumaConf(guest * libvirtxml.Domain){
}

// Set network interface
func (guestconf * GuestConf)setDefaultInterface( guest * libvirtxml.Domain) {
	src_br := &libvirtxml.DomainInterfaceSourceBridge{Bridge: "vmbridge"}
	//tgt_dev := &libvirtxml.DomainInterfaceTarget{Dev: "vnet3"}
	mod := &libvirtxml.DomainInterfaceModel{Type: "virtio"}
	vif := libvirtxml.DomainInterface{
		Source: &libvirtxml.DomainInterfaceSource{Bridge:src_br},
		//Target: tgt_dev,
		Model: mod,
	}
	guest.Devices.Interfaces = [] libvirtxml.DomainInterface{vif}
}

func (guestconf * GuestConf)setUsbConf(guest * libvirtxml.Domain){
	usb := libvirtxml.DomainInput{Type: "tablet", Bus: "usb"}
	guest.Devices.Inputs = []libvirtxml.DomainInput{usb}
}

func (guestconf * GuestConf)setGraphics(guest * libvirtxml.Domain){
	vnc := libvirtxml.DomainGraphicVNC{AutoPort: "yes", Listen:"0.0.0.0",
		Keymap: "en-us"}
	guest.Devices.Graphics = []libvirtxml.DomainGraphic{{VNC: &vnc}}
}

func (guestconf * GuestConf)setDefaultVideo(guest * libvirtxml.Domain) {
	video := libvirtxml.DomainVideo{Model: libvirtxml.DomainVideoModel{Type: "cirrus"}}
	guest.Devices.Videos = append(guest.Devices.Videos, video)
}

func (guestconf * GuestConf)setMemBalloon(guest * libvirtxml.Domain) {
	guest.Devices.MemBalloon = &libvirtxml.DomainMemBalloon{Model: "virtio",
		Stats: &libvirtxml.DomainMemBalloonStats{Period: 10}}
}


func (guestconf * GuestConf)setQemuGuestAgent(guest * libvirtxml.Domain) {
	path := fmt.Sprintf("/var/lib/libvirt/qemu/ckr.qemu.gueset.agent.0-%s.sock", guestconf.Name)
	source := libvirtxml.DomainChardevSource{UNIX: &libvirtxml.DomainChardevSourceUNIX{Mode: "bind",
		Path: path}}
	target := libvirtxml.DomainChannelTarget{
		VirtIO: &libvirtxml.DomainChannelTargetVirtIO{Name: ""}}
	channel := 	libvirtxml.DomainChannel{Source: &source, Target: &target}
	guest.Devices.Channels = append(guest.Devices.Channels, channel)
}

func (guestconf * GuestConf)GetGuestConf(vmid string, conn * libvirt.Connect) error{
	dom, err := conn.LookupDomainByUUIDString(vmid)
	if err != nil {
		return err
	}
	domxml, err := dom.GetXMLDesc(libvirt.DOMAIN_XML_INACTIVE)
	if err != nil {
		return err
	}
	guest := libvirtxml.Domain{}
	guest.Unmarshal(domxml)
	guestconf.Name = guest.Name
	guestconf.Description = guest.Description
	guestconf.UUID = guest.UUID
	guestconf.Mem = guest.Memory.Value
	guestconf.Cpu = guest.VCPU.Value
	//
	guestconf.DiskList = make([]GuestDisk, 0)
	for _, disk := range guest.Devices.Disks {
		if disk.Device != "disk" {
			continue
		}
		var disktype string
		if disk.Source.Network != nil {
			disktype = disk.Source.Network.Protocol
		}
		if disk.Source.Block != nil {
			// TODO SAN & iscsi etc
			disktype = "san"
		}
		if disk.Source.File != nil {
			// Local file
			disktype = "local"
		}
		guestconf.DiskList = append(guestconf.DiskList, GuestDisk{MountPoint: disk.Target.Dev,
			Type: disktype, UUID: disk.Serial})
	}
	guestconf.Arch = X86_64
	return nil
}
