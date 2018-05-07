//
// Copyright (c) 2018 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package virtcontainers

import (
	"encoding/hex"
	"fmt"
	"os"

	govmmQemu "github.com/intel/govmm/qemu"
)

type qemuPpc64le struct {
	// inherit from qemuArchBase, overwrite methods if needed
	qemuArchBase
}

const defaultQemuPath = "/usr/libexec/qemu-kvm"

const defaultQemuMachineType = QemuPpc64le

const defaultQemuMachineOptions = "accel=kvm,usb=off"

const defaultPCBridgeBus = "pci.0"

var qemuPaths = map[string]string{
	QemuPpc64le: defaultQemuPath,
}

var kernelParams = []Param{
	{"tsc", "reliable"},
	{"no_timer_check", ""},
	{"rcupdate.rcu_expedited", "1"},
	{"i8042.direct", "1"},
	{"i8042.dumbkbd", "1"},
	{"i8042.nopnp", "1"},
	{"i8042.noaux", "1"},
	{"noreplace-smp", ""},
	{"reboot", "k"},
	{"console", "hvc0"},
	{"console", "hvc1"},
	{"iommu", "off"},
	{"cryptomgr.notests", ""},
	{"net.ifnames", "0"},
	{"pci", "lastbus=0"},
}

var supportedQemuMachines = []govmmQemu.Machine{
	{
		Type:    QemuPpc64le,
		Options: defaultQemuMachineOptions,
	},
}

// returns the maximum number of vCPUs supported
func maxQemuVCPUs() uint32 {
	return uint32(128)
}

func newQemuArch(config HypervisorConfig) qemuArch {
	machineType := config.HypervisorMachineType
	if machineType == "" {
		machineType = defaultQemuMachineType
	}

	return &qemuPpc64le{
		qemuArchBase{
			machineType:           machineType,
			qemuPaths:             qemuPaths,
			supportedQemuMachines: supportedQemuMachines,
			kernelParamsNonDebug:  kernelParamsNonDebug,
			kernelParamsDebug:     kernelParamsDebug,
			kernelParams:          kernelParams,
		},
	}
}

func (q *qemuPpc64le) capabilities() capabilities {
	var caps capabilities

	// Only pc machine type supports hotplugging drives
	if q.machineType == QemuPpc64le {
		caps.setBlockDeviceHotplugSupport()
	}

	return caps
}

func (q *qemuPpc64le) bridges(number uint32) []Bridge {
	var bridges []Bridge
	var bt bridgeType

	switch q.machineType {

	case QemuPpc64le:
		bt = pciBridge
	default:
		return nil
	}

	for i := uint32(0); i < number; i++ {
		bridges = append(bridges, Bridge{
			Type:    bt,
			ID:      fmt.Sprintf("%s-bridge-%d", bt, i),
			Address: make(map[uint32]string),
		})
	}

	return bridges
}

func (q *qemuPpc64le) cpuModel() string {
        //fmt.Println("HELLLLO")
	cpuModel := defaultCPUModel
	if q.nestedRun {
		cpuModel += ",pmu=off"
	}
	return cpuModel
}

func (q *qemuPpc64le) memoryTopology(memoryMb, hostMemoryMb uint64) govmmQemu.Memory {
	// NVDIMM device needs memory space 1024MB
	// See https://github.com/clearcontainers/runtime/issues/380
	//memoryOffset := 1024

	// add 1G memory space for nvdimm device (vm guest image)
	//memMax := fmt.Sprintf("%dM", hostMemoryMb+uint64(memoryOffset))

	mem := fmt.Sprintf("%dM", memoryMb)

	memory := govmmQemu.Memory{
		Size: mem,
		//Slots:  defaultMemSlots,
		//MaxMem: memMax,
	}

	return memory
}

func (q *qemuPpc64le) appendImage(devices []govmmQemu.Device, path string) ([]govmmQemu.Device, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	//govmmQemu.SCSIController{}
	randBytes, err := generateRandomBytes(8)
	if err != nil {
		return nil, err
	}

	id := makeNameID("image", hex.EncodeToString(randBytes))

	drive := Drive{
		File:   path,
		Format: "raw",
		ID:     id,
	}

	return q.appendBlockDevice(devices, drive), nil
}

// appendBridges appends to devices the given bridges
func (q *qemuPpc64le) appendBridges(devices []govmmQemu.Device, bridges []Bridge) []govmmQemu.Device {
	bus := defaultPCBridgeBus

	for idx, b := range bridges {
		t := govmmQemu.PCIBridge
		if b.Type == pcieBridge {
			t = govmmQemu.PCIEBridge
		}

		devices = append(devices,
			govmmQemu.BridgeDevice{
				Type: t,
				Bus:  bus,
				ID:   b.ID,
				// Each bridge is required to be assigned a unique chassis id > 0
				Chassis: (idx + 1),
				SHPC:    true,
			},
		)
	}

	return devices
}
