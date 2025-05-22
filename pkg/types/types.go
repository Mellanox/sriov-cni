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

package types

import (
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/vishvananda/netlink"
)

const (
	Proto8021q  = "802.1q"
	Proto8021ad = "802.1ad"
)

var VlanProtoInt = map[string]int{Proto8021q: 33024, Proto8021ad: 34984}

// VfState represents the state of the VF
type VfState struct {
	HostIFName   string
	SpoofChk     bool
	Trust        bool
	AdminMAC     string
	EffectiveMAC string
	Vlan         int
	VlanQoS      int
	VlanProto    int
	MinTxRate    int
	MaxTxRate    int
	LinkState    uint32
	MTU          int
}

// FillFromVfInfo - Fill attributes according to the provided netlink.VfInfo struct
func (vs *VfState) FillFromVfInfo(info *netlink.VfInfo) {
	vs.AdminMAC = info.Mac.String()
	vs.LinkState = info.LinkState
	vs.MaxTxRate = int(info.MaxTxRate)
	vs.MinTxRate = int(info.MinTxRate)
	vs.Vlan = info.Vlan
	vs.VlanQoS = info.Qos
	vs.VlanProto = info.VlanProto
	vs.SpoofChk = info.Spoofchk
	vs.Trust = info.Trust != 0
}

type NetConf struct {
	types.NetConf
	SriovNetConf
}

// NetConf extends types.NetConf for sriov-cni
type SriovNetConf struct {
	OrigVfState   VfState // Stores the original VF state as it was prior to any operations done during cmdAdd flow
	DPDKMode      bool    `json:"-"`
	Master        string
	MAC           string
	MTU           *int    // interface MTU
	Vlan          *int    `json:"vlan"`
	VlanQoS       *int    `json:"vlanQoS"`
	VlanProto     *string `json:"vlanProto"` // 802.1ad|802.1q
	DeviceID      string  `json:"deviceID"`  // PCI address of a VF in valid sysfs format
	VFID          int
	MinTxRate     *int   `json:"min_tx_rate"`          // Mbps, 0 = disable rate limiting
	MaxTxRate     *int   `json:"max_tx_rate"`          // Mbps, 0 = disable rate limiting
	SpoofChk      string `json:"spoofchk,omitempty"`   // on|off
	Trust         string `json:"trust,omitempty"`      // on|off
	LinkState     string `json:"link_state,omitempty"` // auto|enable|disable
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`
	LogLevel string `json:"logLevel,omitempty"`
	LogFile  string `json:"logFile,omitempty"`
}

func (n *NetConf) MarshalJSON() ([]byte, error) {
	netConfBytes, err := json.Marshal(&n.NetConf)
	if err != nil {
		return nil, fmt.Errorf("error serializing delegate netConf: %v", err)
	}

	sriovNetConfBytes, err := json.Marshal(&n.SriovNetConf)
	if err != nil {
		return nil, fmt.Errorf("error serializing delegate sriovNetConf: %v", err)
	}

	netConfMap := make(map[string]interface{})
	if err := json.Unmarshal(netConfBytes, &netConfMap); err != nil {
		return nil, err
	}

	sriovNetConfMap := make(map[string]interface{})
	if err := json.Unmarshal(sriovNetConfBytes, &sriovNetConfMap); err != nil {
		return nil, err
	}

	for k, v := range netConfMap {
		sriovNetConfMap[k] = v
	}

	sriovNetConfBytes, err = json.Marshal(sriovNetConfMap)
	if err != nil {
		return nil, err
	}

	return sriovNetConfBytes, nil
}
