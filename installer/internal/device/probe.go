// SPDX-License-Identifier: MIT
//
// Adapted from Maxime Rivest's remagic pure-Go device discovery.
package device

import (
	"bufio"
	"encoding/binary"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

type Probe struct {
	Addr   string
	Gate   string
	Banner string
}

func (p *Probe) IsPaperPro() bool {
	return p.Gate != "" && strings.Contains(strings.ToLower(p.Banner), "dropbear")
}

func (p *Probe) IsDropbear() bool {
	return strings.Contains(strings.ToLower(p.Banner), "dropbear")
}

func ProbeAddr(address string, timeout time.Duration) (*Probe, error) {
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(address, "22"), timeout)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(timeout))
	probe := &Probe{Addr: address}
	reader := bufio.NewReader(connection)
	for range 3 {
		line, readError := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SSH-") {
			probe.Banner = line
			break
		}
		if line != "" && probe.Gate == "" {
			probe.Gate = line
		}
		if readError != nil {
			break
		}
	}
	return probe, nil
}

func Discover(timeout time.Duration) []*Probe {
	addresses := append([]string{DefaultUSBAddr}, lanHosts()...)
	seen := make(map[string]bool)
	var (
		mu      sync.Mutex
		results []*Probe
		group   sync.WaitGroup
	)
	semaphore := make(chan struct{}, 192)
	for _, address := range addresses {
		if seen[address] {
			continue
		}
		seen[address] = true
		group.Add(1)
		semaphore <- struct{}{}
		go func() {
			defer group.Done()
			defer func() { <-semaphore }()
			probe, err := ProbeAddr(address, timeout)
			if err != nil || !probe.IsDropbear() {
				return
			}
			mu.Lock()
			results = append(results, probe)
			mu.Unlock()
		}()
	}
	group.Wait()
	sort.Slice(results, func(left, right int) bool {
		return results[left].Addr < results[right].Addr
	})
	return results
}

func lanHosts() []string {
	var hosts []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, current := range interfaces {
		if current.Flags&net.FlagUp == 0 || current.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := current.Addrs()
		for _, address := range addresses {
			network, ok := address.(*net.IPNet)
			if !ok {
				continue
			}
			ip := network.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			ones, _ := network.Mask.Size()
			if ones < 24 {
				ones = 24
			}
			mask := net.CIDRMask(ones, 32)
			base := binary.BigEndian.Uint32(ip.Mask(mask))
			self := binary.BigEndian.Uint32(ip)
			count := uint32(1) << (32 - ones)
			for offset := uint32(1); offset < count-1; offset++ {
				host := base + offset
				if host == self {
					continue
				}
				candidate := make(net.IP, 4)
				binary.BigEndian.PutUint32(candidate, host)
				hosts = append(hosts, candidate.String())
			}
		}
	}
	return hosts
}
