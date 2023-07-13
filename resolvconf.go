package main

import (
	"bufio"
	"os"
	"strings"
)

// change /etc/resolv.conf so that it uses the given resolver;
// success: return (old content, upstream resolver, nil)
// failed to write: return (old content, upstream resolver, err)
// failed to read: return ("", "", err)
func UpdateResolvConf(resolver string) (string, string, error) {
	f, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	newContent := "# created by dnstweak\n"
	newContent += "nameserver " + resolver + "\n"

	oldResolver := ""
	oldLines := make([]string, 0)
	scanner := bufio.NewScanner(f)
	seenDnsTweak := false
	for scanner.Scan() {
		line := scanner.Text()
		oldLines = append(oldLines, line)
		line = strings.TrimLeft(line, " ")
		if oldResolver == "" && strings.HasPrefix(line, "nameserver ") {
			oldResolver = strings.TrimPrefix(line, "nameserver ") + ":53"
		}

		if strings.HasPrefix(line, "search ") {
			newContent += line + "\n"
		}

		if strings.HasPrefix(line, "#dnstweak#") {
			seenDnsTweak = true
		}
	}

	if seenDnsTweak {
		// if the old content was from dnstweak output, then strip it down to just
		// what was there before dnstweak wrote it
		oldLines2 := make([]string, 0)
		for _, line := range oldLines {
			if strings.HasPrefix(line, "#dnstweak#") {
				oldLines2 = append(oldLines2, strings.TrimPrefix(line, "#dnstweak#"))
			}
		}
		oldLines = oldLines2
	}

	oldContent := strings.Join(oldLines, "\n") + "\n"

	// append the old content to the new content, with "#dnstweak#", so that it can be restored later
	for _, line := range oldLines {
		newContent += "#dnstweak#" + line + "\n"
	}

	err = os.WriteFile("/etc/resolv.conf", []byte(newContent), 0644)
	return oldContent, oldResolver, err
}

func RestoreResolvConf(contents string) error {
	err := os.WriteFile("/etc/resolv.conf", []byte(contents), 0644)
	return err
}
