// Copyright (c) seasonjs. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

//go:build windows && amd64

package sd

import (
	_ "embed"
	"encoding/json"
	"golang.org/x/sys/cpu"
	"log"
	"os/exec"
	"strings"
)

//go:embed deps/windows/sd-abi_avx2.dll
var libStableDiffusionAvx2 []byte

//go:embed deps/windows/sd-abi_avx.dll
var libStableDiffusionAvx []byte

//go:embed deps/windows/sd-abi_avx512.dll
var libStableDiffusionAvx512 []byte

//go:embed deps/windows/sd-abi_cuda12.dll
var libStableDiffusionCuda12 []byte

var libName = "stable-diffusion-*.dll"

func getDl(gpu bool) []byte {
	if gpu {
		info, err := NewGPU()
		if err != nil {
			log.Println(err)
		}
		driver := info.Cuda()
		log.Print("get gpu info: ", driver.Name)

		if driver.Available() {
			log.Println("Use GPU CUDA instead.")
			return libStableDiffusionCuda12
		}

		log.Println("GPU not support, use CPU instead.")
	}

	if cpu.X86.HasAVX512 {
		log.Println("Use CPU AVX512 instead.")
		return libStableDiffusionAvx512
	}

	if cpu.X86.HasAVX2 {
		log.Println("Use CPU AVX2 instead.")
		return libStableDiffusionAvx2
	}

	if cpu.X86.HasAVX {
		log.Println("Use CPU AVX instead.")
		return libStableDiffusionAvx
	}

	panic("Automatic loading of dynamic library failed, please use `NewRwkvModel` method load manually. ")
}

func runPowerShellCommand(command string) (string, error) {
	cmd := exec.Command("powershell", "-Command", command)

	// execute the command and get the output
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

type Driver struct {
	Name                 string `json:"Name"`
	AdapterCompatibility string `json:"AdapterCompatibility"`
	AdapterRAM           string `json:"AdapterRAM"`
}

func (d *Driver) Available() bool {
	return d.Name != "" && d.AdapterCompatibility != "" && d.AdapterRAM != ""
}

// GPU 类用于管理显卡信息
type GPU struct {
	drivers []Driver
	cuda    *Driver
	rocm    *Driver
}

func NewGPU() (*GPU, error) {
	cmd := exec.Command("powershell", `
        $graphicsCards = Get-WmiObject Win32_VideoController
        $graphicsArray = @()
        foreach ($card in $graphicsCards) {
            $graphicsInfo = @{
                'Name'                 = $card.Caption
                'AdapterCompatibility' = $card.VideoProcessor
                'AdapterRAM'           = $card.AdapterRAM
            }
            $graphicsArray += $graphicsInfo
        }
        $graphicsArray | ConvertTo-Json
    `)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var drivers []Driver
	err = json.Unmarshal(output, &drivers)
	if err != nil {
		return nil, err
	}

	cudaSupport := &Driver{}
	rocmSupport := &Driver{}

	for _, driver := range drivers {
		if strings.Contains(strings.ToUpper(driver.Name), "NVIDIA") {
			cudaSupport = &driver
		} else if strings.Contains(strings.ToUpper(driver.Name), "AMD") {
			rocmSupport = &driver
		}
	}

	return &GPU{
		drivers: drivers,
		cuda:    cudaSupport,
		rocm:    rocmSupport,
	}, nil
}

func (g *GPU) Cuda() *Driver {
	return g.cuda
}

func (g *GPU) ROCm() *Driver {
	return g.rocm
}

func (g *GPU) Info() []Driver {
	return g.drivers
}
