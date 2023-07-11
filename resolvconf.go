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
	content := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		content += line + "\n"
		line = strings.TrimLeft(line, " ")
		if oldResolver == "" && strings.HasPrefix(line, "nameserver ") {
			oldResolver = strings.TrimPrefix(line, "nameserver ") + ":53"
		}

		if strings.HasPrefix(strings.TrimLeft(line, " "), "search ") {
			newContent += line + "\n"
		}
	}

	// TODO: put oldContent in but commented out, to help users restore it?
	// how do we make sure we don't end up with 100 copies of it concatenated?
	// or copy it to a backup file?

	err = os.WriteFile("/etc/resolv.conf", []byte(newContent), 0644)
	return content, oldResolver, err
}

func RestoreResolvConf(contents string) error {
	err := os.WriteFile("/etc/resolv.conf", []byte(contents), 0644)
	return err
}
