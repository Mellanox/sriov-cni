// Copyright 2025 sriov-cni authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sriovtypes "github.com/k8snetworkplumbingwg/sriov-cni/pkg/types"
)

var (
	sriovConfigured = "/sriov_numvfs"
	// NetDirectory sysfs net directory
	NetDirectory = "/sys/class/net"
	// SysBusPci is sysfs pci device directory
	SysBusPci = "/sys/bus/pci/devices"
	// SysV4ArpNotify is the sysfs IPv4 ARP Notify directory
	SysV4ArpNotify = "/proc/sys/net/ipv4/conf/"
	// SysV6NdiscNotify is the sysfs IPv6 Neighbor Discovery Notify directory
	SysV6NdiscNotify = "/proc/sys/net/ipv6/conf/"
	// UserspaceDrivers is a list of driver names that don't have netlink representation for their devices
	UserspaceDrivers = []string{"vfio-pci", "uio_pci_generic", "igb_uio"}
)

// EnableArpAndNdiscNotify enables IPv4 arp_notify and IPv6 ndisc_notify for netdev
func EnableArpAndNdiscNotify(ifName string) error {
	/* For arp_notify, when a value of "1" is set then a Gratuitous ARP request will be sent
	 * when the network device is brought up or if the link-layer address changes.
	 * For ndsic_notify, when a value of "1" is set then a Unsolicited Neighbor Advertisement
	 * will be sent when the network device is brought up or if the link-layer address changes.
	 * Both of these being enabled would be useful in the case when an application reenables
	 * an interface or if the MAC address configuration is changed. The kernel is responsible
	 * for sending of these packets when the conditions are met.
	 */
	v4ArpNotifyPath := filepath.Join(SysV4ArpNotify, ifName, "arp_notify")
	err := os.WriteFile(v4ArpNotifyPath, []byte("1"), os.ModeAppend)
	if err != nil {
		return fmt.Errorf("failed to write arp_notify=1 for interface %s: %v", ifName, err)
	}
	v6NdiscNotifyPath := filepath.Join(SysV6NdiscNotify, ifName, "ndisc_notify")
	err = os.WriteFile(v6NdiscNotifyPath, []byte("1"), os.ModeAppend)
	if err != nil {
		return fmt.Errorf("failed to write ndisc_notify=1 for interface %s: %v", ifName, err)
	}
	return nil
}

// EnableOptimisticDad enables IPv6 /proc/sys/net/ipv6/conf/$ifName/optimistic_dad
func EnableOptimisticDad(ifName string) error {
	path := filepath.Join(SysV6NdiscNotify, ifName, "optimistic_dad")
	err := os.WriteFile(path, []byte("1"), os.ModeAppend)
	if err != nil {
		return fmt.Errorf("failed to write optimistic_dad=1 for interface %s: %v", ifName, err)
	}
	return nil
}

// GetSriovNumVfs takes in a PF name(ifName) as string and returns number of VF configured as int
func GetSriovNumVfs(ifName string) (int, error) {
	var vfTotal int

	sriovFile := filepath.Join(NetDirectory, ifName, "device", sriovConfigured)
	if _, err := os.Lstat(sriovFile); err != nil {
		return vfTotal, fmt.Errorf("failed to open the sriov_numfs of device %q: %v", ifName, err)
	}

	data, err := os.ReadFile(sriovFile)
	if err != nil {
		return vfTotal, fmt.Errorf("failed to read the sriov_numfs of device %q: %v", ifName, err)
	}

	if len(data) == 0 {
		return vfTotal, fmt.Errorf("no data in the file %q", sriovFile)
	}

	sriovNumfs := strings.TrimSpace(string(data))
	vfTotal, err = strconv.Atoi(sriovNumfs)
	if err != nil {
		return vfTotal, fmt.Errorf("failed to convert sriov_numfs(byte value) to int of device %q: %v", ifName, err)
	}

	return vfTotal, nil
}

// GetVfid takes in VF's PCI address(addr) and pfName as string and returns VF's ID as int
func GetVfid(addr, pfName string) (int, error) {
	var id int
	vfTotal, err := GetSriovNumVfs(pfName)
	if err != nil {
		return id, err
	}
	for vf := 0; vf < vfTotal; vf++ {
		vfDir := filepath.Join(NetDirectory, pfName, "device", fmt.Sprintf("virtfn%d", vf))
		_, err := os.Lstat(vfDir)
		if err != nil {
			continue
		}
		pciinfo, err := os.Readlink(vfDir)
		if err != nil {
			continue
		}
		pciaddr := filepath.Base(pciinfo)
		if pciaddr == addr {
			return vf, nil
		}
	}
	return id, fmt.Errorf("unable to get VF ID with PF: %s and VF pci address %v", pfName, addr)
}

// GetPfName returns PF net device name of a given VF pci address
func GetPfName(vf string) (string, error) {
	pfSymLink := filepath.Join(SysBusPci, vf, "physfn", "net")
	_, err := os.Lstat(pfSymLink)
	if err != nil {
		return "", err
	}

	files, err := os.ReadDir(pfSymLink)
	if err != nil {
		return "", err
	}

	if len(files) < 1 {
		return "", fmt.Errorf("PF network device not found")
	}

	return strings.TrimSpace(files[0].Name()), nil
}

// GetPciAddress takes in a interface(ifName) and VF id and returns its pci addr as string
func GetPciAddress(ifName string, vf int) (string, error) {
	var pciaddr string
	vfDir := filepath.Join(NetDirectory, ifName, "device", fmt.Sprintf("virtfn%d", vf))
	dirInfo, err := os.Lstat(vfDir)
	if err != nil {
		return pciaddr, fmt.Errorf("can't get the symbolic link of virtfn%d dir of the device %q: %v", vf, ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pciaddr, fmt.Errorf("no symbolic link for the virtfn%d dir of the device %q", vf, ifName)
	}

	pciinfo, err := os.Readlink(vfDir)
	if err != nil {
		return pciaddr, fmt.Errorf("can't read the symbolic link of virtfn%d dir of the device %q: %v", vf, ifName, err)
	}

	pciaddr = filepath.Base(pciinfo)
	return pciaddr, nil
}

// GetSharedPF takes in VF name(ifName) as string and returns the other VF name that shares same PCI address as string
func GetSharedPF(ifName string) (string, error) {
	pfName := ""
	pfDir := filepath.Join(NetDirectory, ifName)
	dirInfo, err := os.Lstat(pfDir)
	if err != nil {
		return pfName, fmt.Errorf("can't get the symbolic link of the device %q: %v", ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pfName, fmt.Errorf("no symbolic link for dir of the device %q", ifName)
	}

	fullpath, _ := filepath.EvalSymlinks(pfDir)
	parentDir := fullpath[:len(fullpath)-len(ifName)]
	dirList, _ := os.ReadDir(parentDir)

	for _, file := range dirList {
		if file.Name() != ifName {
			pfName = file.Name()
			return pfName, nil
		}
	}

	return pfName, fmt.Errorf("shared PF not found")
}

// GetVFLinkName returns VF's network interface name given it's PCI addr
func GetVFLinkName(pciAddr string) (string, error) {
	var names []string
	vfDir := filepath.Join(SysBusPci, pciAddr, "net")
	if _, err := os.Lstat(vfDir); err != nil {
		return "", err
	}

	fInfos, err := os.ReadDir(vfDir)
	if err != nil {
		return "", fmt.Errorf("failed to read net dir of the device %s: %v", pciAddr, err)
	}

	if len(fInfos) == 0 {
		return "", fmt.Errorf("VF device %s sysfs path (%s) has no entries", pciAddr, vfDir)
	}

	names = make([]string, len(fInfos))
	for idx, f := range fInfos {
		names[idx] = f.Name()
	}

	if len(names) < 1 {
		return "", fmt.Errorf("VF device %s has no entries", pciAddr)
	}
	return names[0], nil
}

// GetVFLinkNamesFromVFID returns VF's network interface name given it's PF name as string and VF id as int
func GetVFLinkNamesFromVFID(pfName string, vfID int) ([]string, error) {
	vfDir := filepath.Join(NetDirectory, pfName, "device", fmt.Sprintf("virtfn%d", vfID), "net")
	if _, err := os.Lstat(vfDir); err != nil {
		return nil, err
	}

	fInfos, err := os.ReadDir(vfDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read the virtfn%d dir of the device %q: %v", vfID, pfName, err)
	}

	names := make([]string, 0)
	for _, f := range fInfos {
		names = append(names, f.Name())
	}

	return names, nil
}

// HasDpdkDriver checks if a device is attached to dpdk supported driver
func HasDpdkDriver(pciAddr string) (bool, error) {
	driverLink := filepath.Join(SysBusPci, pciAddr, "driver")
	driverPath, err := filepath.EvalSymlinks(driverLink)
	if err != nil {
		return false, err
	}
	driverStat, err := os.Stat(driverPath)
	if err != nil {
		return false, err
	}
	driverName := driverStat.Name()
	for _, drv := range UserspaceDrivers {
		if driverName == drv {
			return true, nil
		}
	}
	return false, nil
}

// SaveNetConf takes in container ID, data dir and Pod interface name as string and a json encoded struct Conf
// and save this Conf in data dir
func SaveNetConf(cid, dataDir, podIfName string, netConf *sriovtypes.NetConf) error {
	netConfBytes, err := json.Marshal(netConf)
	if err != nil {
		return fmt.Errorf("error serializing delegate netConf: %v", err)
	}

	s := []string{cid, podIfName}
	cRef := strings.Join(s, "-")

	// save the rendered netconf for cmdDel
	return saveScratchNetConf(cRef, dataDir, netConfBytes)
}

func saveScratchNetConf(containerID, dataDir string, netconf []byte) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return fmt.Errorf("failed to create the sriov data directory(%q): %v", dataDir, err)
	}

	path := filepath.Join(dataDir, containerID)

	err := os.WriteFile(path, netconf, 0600)
	if err != nil {
		return fmt.Errorf("failed to write container data in the path(%q): %v", path, err)
	}

	return err
}

// ReadScratchNetConf takes in container ID, Pod interface name and data dir as string and returns a pointer to Conf
func ReadScratchNetConf(cRefPath string) ([]byte, error) {
	data, err := os.ReadFile(cRefPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read container data in the path(%q): %v", cRefPath, err)
	}

	return data, err
}

// CleanCachedNetConf removed cached NetConf from disk
func CleanCachedNetConf(cRefPath string) error {
	if err := os.Remove(cRefPath); err != nil {
		return fmt.Errorf("error removing NetConf file %s: %v", cRefPath, err)
	}
	return nil
}

// SetVFEffectiveMAC will try to set the mac address on a specific VF interface
//
// the function will also validate that the mac address was configured as expect
// it will return an error if it didn't manage to configure the vf mac address
// or the mac is not equal to the expect one
// retries 20 times and wait 100 milliseconds
//
// Some NIC drivers (i.e. i40e/iavf) set VF MAC address asynchronously
// via PF. This means that while the PF could already show the VF with
// the desired MAC address, the netdev VF may still have the original
// one. If in this window we issue a netdev VF MAC address set, the driver
// will return an error and the pod will fail to create.
// Other NICs (Mellanox) require explicit netdev VF MAC address so we
// cannot skip this part.
// Retry up to 5 times; wait 200 milliseconds between retries
func SetVFEffectiveMAC(netLinkManager NetlinkManager, netDeviceName, macAddress string) error {
	hwaddr, err := net.ParseMAC(macAddress)
	if err != nil {
		return fmt.Errorf("failed to parse MAC address %s: %v", macAddress, err)
	}

	orgLinkObj, err := netLinkManager.LinkByName(netDeviceName)
	if err != nil {
		return err
	}

	return Retry(20, 100*time.Millisecond, func() error {
		if err := netLinkManager.LinkSetHardwareAddr(orgLinkObj, hwaddr); err != nil {
			return err
		}

		linkObj, err := netLinkManager.LinkByName(netDeviceName)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", orgLinkObj.Attrs().Name, err)
		}
		if linkObj.Attrs().HardwareAddr.String() != macAddress {
			return fmt.Errorf("effective mac address is different from requested one")
		}

		return nil
	})
}

// SetVFHardwareMAC will try to set the hardware mac address on a specific VF ID under a requested PF

// the function will also validate that the mac address was configured as expect
// it will return an error if it didn't manage to configure the vf mac address
// or the mac is not equal to the expect one
// retries 20 times and wait 100 milliseconds
func SetVFHardwareMAC(netLinkManager NetlinkManager, pfDevice string, vfID int, macAddress string) error {
	hwaddr, err := net.ParseMAC(macAddress)
	if err != nil {
		return fmt.Errorf("failed to parse MAC address %s: %v", macAddress, err)
	}

	orgLinkObj, err := netLinkManager.LinkByName(pfDevice)
	if err != nil {
		return err
	}

	return Retry(20, 100*time.Millisecond, func() error {
		if err := netLinkManager.LinkSetVfHardwareAddr(orgLinkObj, vfID, hwaddr); err != nil {
			return err
		}

		linkObj, err := netLinkManager.LinkByName(pfDevice)
		if err != nil {
			return fmt.Errorf("failed to get netlink device with name %s: %q", orgLinkObj.Attrs().Name, err)
		}
		if linkObj.Attrs().Vfs[vfID].Mac.String() != macAddress {
			return fmt.Errorf("hardware mac address is different from requested one")
		}

		return nil
	})
}

// IsValidMACAddress checks if net.HardwareAddr is a valid MAC address.
func IsValidMACAddress(addr net.HardwareAddr) bool {
	invalidMACAddresses := [][]byte{
		{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	}
	valid := false
	if len(addr) == 6 {
		valid = true
		for _, invalidMACAddress := range invalidMACAddresses {
			if bytes.Equal(addr, invalidMACAddress) {
				valid = false
				break
			}
		}
	}
	return valid
}

// IsIPv4 checks if a net.IP is an IPv4 address.
func IsIPv4(ip net.IP) bool {
	return ip.To4() != nil
}

// IsIPv6 checks if a net.IP is an IPv6 address.
func IsIPv6(ip net.IP) bool {
	return ip.To4() == nil && ip.To16() != nil
}

// Retry retries a given function until no return error; times out after retries*sleep
func Retry(retries int, sleep time.Duration, f func() error) error {
	err := error(nil)
	for retry := 0; retry < retries; retry++ {
		err = f()
		if err == nil {
			return nil
		}
		time.Sleep(sleep)
	}
	return err
}
