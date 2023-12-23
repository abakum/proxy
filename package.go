package proxy

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Helps vncaddrbook use proxy from environment for call of vncviewer
// usage: `vncaddrbook.exe -ViewerPath proxy.exe`
// case `vncviewer' is empty then `vncviewer` is `.\vncviewer`
// case $http_proxy or $all_proxy has `socks://:1080“ then by double click on item.vnc
// instead run `vncviewer.exe -config addresbook\item.vnc`
// or with passwd `vncviewer.exe -console -passwd - -config addresbook\item.vnc`
// will be run `vncviewer.exe -config addresbook\item.vnc -ProxyType socks -ProxyServer :1080`
// or with passwd `vncviewer.exe -console -passwd - -config addresbook\item.vnc -ProxyType socks -ProxyServer :1080`
func VncAddrBook(vncviewer string) {
	//netsh winhttp set proxy proxy-server="socks=localhost:1080" bypass-list="localhost"
	const (
		SOCKS   = "socks"
		CONSOLE = "-console"
	)
	var (
		stdin        io.WriteCloser
		errStdinPipe error
		passwd       bool
		err          error
		vncViewer    = fmt.Sprintf(".%cvncviewer", os.PathSeparator)
	)
	if len(os.Args) > 1 {
		passwd = os.Args[1] == CONSOLE
		if passwd || os.Args[1] == "-config" {
		} else {
			return
		}
	} else {
		return
	}
	args := os.Args[1:]
	proxy := get("http_proxy", "all_proxy") // socks://127.0.0.1:1080
	if proxy != "" {
		shp, err := url.Parse(proxy)
		if err == nil {
			ProxyType := "httpconnect"
			if strings.HasPrefix(shp.Scheme, SOCKS) {
				ProxyType = SOCKS
			}
			args = append(args, "-ProxyType="+ProxyType)
			args = append(args, "-ProxyServer="+shp.Host)
		}
	}
	if vncviewer == "" {
		vncviewer = vncViewer
	}
	cmd := exec.Command(vncviewer, args...)
	if passwd {
		stdin, errStdinPipe = cmd.StdinPipe()
	}
	err = cmd.Start()
	fmt.Println(cmd.Args, err, cmd.Dir)
	if passwd && err == nil && errStdinPipe == nil {
		io.Copy(stdin, os.Stdin) // transfers passwd from vncaddrbook to vncviewer
	}
	os.Exit(0)
}

func get(keys ...string) (val string) {
	var (
		reg registry.Key
		err error
	)
	errReg := fmt.Errorf("")
	if runtime.GOOS == "windows" {
		// setx key val
		reg, errReg = registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
		if errReg == nil {
			defer reg.Close()
		}
	}
	for _, key := range keys {
		val = os.Getenv(key)
		if val != "" {
			return
		}
		val = os.Getenv(strings.ToUpper(key))
		if val != "" {
			return
		}
		if errReg == nil {
			val, _, err = reg.GetStringValue(key)
			if err == nil && val != "" {
				return
			}
			val, _, _ = reg.GetStringValue(strings.ToUpper(key))
		}
	}
	return
}
