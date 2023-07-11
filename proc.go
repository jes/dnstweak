package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func FindProcess(addr *net.UDPAddr) string {
	// look in /proc/net/udp to find the inode of this address, then look
	// through all of the /proc/$pid/fd/* links to find which one corresponds
	// to this socket, and then look in /proc/$pid/cmdline to find the name
	// of the process

	inode, ok := FindInode(addr)
	if !ok {
		return ""
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Printf("%v\n", err)
		return ""
	}

	for _, e := range entries {
		if e.Name() == "1" {
			continue
		}
		if isPID(e.Name()) {
			pid, _ := strconv.Atoi(e.Name())
			if HasInode(pid, inode) {
				// XXX: race window here
				return ProcessName(pid)
			}
		}
	}

	return ""
}

func FindInode(addr *net.UDPAddr) (uint64, bool) {
	// look in /proc/net/udp to find the inode for this address

	f, err := os.Open("/proc/net/udp")
	if err != nil {
		log.Printf("%v\n", err)
		return 0, false
	}
	defer f.Close()

	addrIpInt := int64(addr.IP[0]) + int64(addr.IP[1])*256 + int64(addr.IP[2])*256*256 + int64(addr.IP[3])*256*256*256

	scanner := bufio.NewScanner(f)
	_ = scanner.Scan() // skip first line
	for scanner.Scan() {
		// e.g. " 9777: 0100007F:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000     0        0 2969713 2 ffff9ce6fd1cb180 0"
		line := scanner.Text()
		line = strings.TrimLeft(line, " \t")
		cols := strings.Fields(line)
		ipHex, portHex, ok := strings.Cut(cols[1], ":")
		if !ok {
			log.Printf("/proc/net/udp does not appear to be in the expected format (\"%s\" should look like \"0100007F:9A2A\")\n", cols[1])
			continue
		}
		ipInt, err := strconv.ParseInt(ipHex, 16, 64)
		if err != nil {
			log.Printf("%v\n", err)
			continue
		}
		port, err := strconv.ParseInt(portHex, 16, 0)
		if err != nil {
			log.Printf("%v\n", err)
			continue
		}
		if int(port) == addr.Port && ipInt == addrIpInt {
			inode, err := strconv.ParseUint(cols[9], 10, 64)
			if err != nil {
				log.Printf("/proc/net/udp does not appear to be in the expected format (\"%s\" should be a number)\n", cols[9])
			} else {
				return inode, true
			}
		}
	}

	return 0, false
}

func isPID(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func HasInode(pid int, inode uint64) bool {
	// look under /proc/$pid/fd/ to find a link to the socket inode

	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		log.Printf("%v\n", err)
		return false
	}

	for _, fd := range fds {
		fdPath := fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name())
		stat, err := os.Stat(fdPath)
		if err != nil {
			switch err.(type) {
			case *fs.PathError:
				// this is common, don't bother printing anything
				break
			default:
				log.Printf("%v\n", err)
			}
			continue
		}
		data := stat.Sys()
		switch data.(type) {
		case *syscall.Stat_t:
			s := data.(*syscall.Stat_t)
			if s.Ino == inode {
				return true
			}
		}
	}
	return false
}

func ProcessName(pid int) string {
	// look in /proc/$pid/cmdline

	cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		log.Printf("%v\n", err)
		return fmt.Sprintf("<unknown>/%d", pid)
	}

	cmdline := string(cmdlineBytes[:])
	prog, _, found := strings.Cut(cmdline, "\x00")
	if !found {
		return fmt.Sprintf("<unknown>/%d", pid)
	}

	pathParts := strings.Split(prog, "/")
	return fmt.Sprintf("%s/%d", pathParts[len(pathParts)-1], pid)
}
