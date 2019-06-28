package main

import (
	"log"
	"syscall"

	"github.com/klauspost/cpuid"
)

func plural(n int, fill string) string {
	if n == 1 {
		return fill
	}
	return "s"
}

func println(v ...interface{}) {
	if *flagLog != "" {
		log.Println(v...)
	}
}

func printf(f string, v ...interface{}) {
	if *flagLog != "" {
		log.Printf(f, v...)
	}
}

func detailCPU() {
	printf("processor information")
	printf("  name: %s", cpuid.CPU.BrandName)
	printf("  physical cores: %d", cpuid.CPU.PhysicalCores)
	printf("  threads per core: %d", cpuid.CPU.ThreadsPerCore)
	printf("  logical cores: %d", cpuid.CPU.LogicalCores)
	printf("  family %d model %d", cpuid.CPU.Family, cpuid.CPU.Model)
	printf("  features: %v", cpuid.CPU.Features)
	printf("  cache line bytes: %d", cpuid.CPU.CacheLine)
	printf("  L1 cache: %d + %d bytes (instruction + data)", cpuid.CPU.Cache.L1I, cpuid.CPU.Cache.L1D)
	printf("  L2 unified cache: %d bytes", cpuid.CPU.Cache.L2)
	printf("  L3 unified cache %d bytes:", cpuid.CPU.Cache.L3)
}

func getResourceUsage() (user, system float64, size uint64) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		println("Error: unable to gather resource usage data:", err)
	}
	user = float64(usage.Utime.Sec) + float64(usage.Utime.Usec)/1e6   // work by this process
	system = float64(usage.Stime.Sec) + float64(usage.Stime.Usec)/1e6 // work by OS on behalf of this process (reading files)
	size = uint64(uint32(usage.Maxrss))
	return
}
