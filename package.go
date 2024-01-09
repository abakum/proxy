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
// case $all_proxy has `socks://localhost:1080` then by double click on item.vnc
// instead runing `vncviewer.exe -config addresbook\item.vnc`
// will be run `vncviewer.exe -config addresbook\item.vnc -ProxyType=socks -ProxyServer=localhost:1080`
// case $http_proxy has `http://localhost:3128` then by double click on itemWithPasswd.vnc
// instead runing `vncviewer.exe -console -passwd - -config addresbook\itemWithPasswd.vnc`
// will be run `vncviewer.exe -console -passwd - -config addresbook\itemWithPasswd.vnc -ProxyType=httpconnect -ProxyServer=localhost:3128`
// case global is true then only `setX key val` or `SETENV key val` will be using
// or `netsh winhttp set proxy proxy-server="http://localhost:3128;socks=localhost:1080"`
// for unset http_proxy use `REG delete HKCU\Environment /F /V http_proxy`
func VncAddrBook(global bool, vncviewer string) {
	// netsh winhttp set proxy [proxy-server=]host[:port] [bypass-list=]"<local>;bar"
	// netsh winhttp set proxy host[:port] as proxy-server="http=host[:port]"

	// `netsh winhttp set proxy proxy-server="http=localhost:3128;socks=localhost:1080"`
	// to do: parse bypass-list="<local>;bar;*.foo.com"
	var (
		stdin        io.WriteCloser
		errStdinPipe error
		passwd       bool
		err          error
		vncViewer    = fmt.Sprintf(".%cvncviewer", os.PathSeparator)
	)
	if len(os.Args) > 1 {
		passwd = os.Args[1] == "-console"
		if passwd || os.Args[1] == "-config" {
		} else {
			return
		}
	} else {
		return
	}
	args := os.Args[1:]
	http, _, _, socks := GetProxy()
	fmt.Println("GetProxy", http, socks)
	suff := SetProxy(global, "all_proxy", "socks", socks, "socks", "1080")
	if len(suff) > 0 {
		args = append(args, suff...)
	} else {
		suff = SetProxy(global, "http_proxy", "httpconnect", http, "http", "3128")
		if len(suff) > 0 {
			args = append(args, suff...)
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

// get proxy from env
func SetProxy(global bool, env, ProxyType, ProxyServer, scheme, port string) (suff []string) {
	if ProxyServer == "" {
		proxy := GetX(global, env)
		fmt.Println("GetX", proxy)
		if !strings.HasPrefix(proxy, scheme) {
			return
		} else {
			shp, err := url.Parse(proxy)
			if err == nil {
				ProxyServer = shp.Host
			} else {
				ProxyServer = "127.0.0.1:" + port
			}
		}
	}
	suff = append(suff, "-ProxyType="+ProxyType, "-ProxyServer="+ProxyServer)
	return
}

// case global is true then only `setX key val` will be get
func GetX(global bool, key string) (val string) {
	var (
		reg registry.Key
		err error
	)
	errReg := fmt.Errorf("no global")
	if runtime.GOOS == "windows" {
		// setx key val
		reg, errReg = registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
		if errReg == nil {
			defer reg.Close()
		}
	}
	if !global {
		val = os.Getenv(key)
		if val != "" {
			return
		}
		val = os.Getenv(strings.ToUpper(key))
		if val != "" {
			return
		}
	}
	if errReg == nil {
		val, _, err = reg.GetStringValue(key)
		if err == nil && val != "" {
			return
		}
		val, _, _ = reg.GetStringValue(strings.ToUpper(key))
	}
	return
}

// get proxy from ie
func GetProxy() (http, https, ftp, socks string) {
	// `netsh winhttp import proxy source=ie`
	if runtime.GOOS != "windows" {
		return
	}
	reg, err := registry.OpenKey(registry.CURRENT_USER,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer reg.Close()
	ProxyEnable, _, err := reg.GetIntegerValue("ProxyEnable")
	if err != nil || ProxyEnable == 0 {
		return
	}
	ProxyServer, _, err := reg.GetStringValue("ProxyServer")
	if err != nil {
		return
	}
	for _, proxy := range strings.Split(ProxyServer, ";") {
		kv := strings.Split(proxy, "=")
		if len(kv) < 2 {
			http = proxy
			continue
		}
		switch kv[0] {
		case "http":
			http = kv[1]
		case "https":
			https = kv[1]
		case "ftp":
			ftp = kv[1]
		case "socks":
			socks = kv[1]
		}
	}
	return
}

// set string values by path
func SetStringValues(k registry.Key, path string, mnv map[string]string) {
	key, err := registry.OpenKey(k, path, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	for name, val := range mnv {
		key.SetStringValue(name, val)
	}
}

// set RealVNC proxy
func RealSet(ProxyType, ProxyServer string) {
	SetStringValues(registry.CURRENT_USER, `SOFTWARE\RealVNC\vncviewer`, map[string]string{
		"ProxyType":   ProxyType,
		"ProxyServer": ProxyServer,
	})
}

// get string values by path
func GetStringValues(k registry.Key, path string, names ...string) (vals []string) {
	vals = make([]string, len(names))
	key, err := registry.OpenKey(k, path, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	for i, name := range names {
		vals[i], _, _ = key.GetStringValue(name)
	}
	return
}

// get RealVNC proxy
func RealGet() (ProxyType, ProxyServer string) {
	vals := GetStringValues(registry.CURRENT_USER, `SOFTWARE\RealVNC\vncviewer`, "ProxyType", "ProxyServer")
	return vals[0], vals[1]
}

// if in the file .vnc from the address book the `ProxyServer` is empty, then `vncviewer` does not use the `ProxyServer` from the registry
//
// RealAddrBook(vncviewer) forcibly launches `vncviewer` from the address book using a "ProxyServer" from the registry
//
// place `proxy.exe` next to RealAddrBook.exe then run `RealAddrBook.exe -ViewerPath=proxy.exe` to use the `ProxyServer` from the registry
func RealAddrBook(vncviewer string) {
	var (
		stdin        io.WriteCloser
		errStdinPipe error
		passwd       bool
		err          error
		vncViewer    = fmt.Sprintf(".%cvncviewer", os.PathSeparator)
	)
	if len(os.Args) > 1 {
		passwd = os.Args[1] == "-console"
		if passwd || os.Args[1] == "-config" {
		} else {
			return
		}
	} else {
		return
	}
	args := os.Args[1:]
	ProxyType, ProxyServer := RealGet()
	if ProxyServer != "" {
		args = append(args, "-ProxyServer="+ProxyServer)
		if ProxyType != "" {
			args = append(args, "-ProxyType="+ProxyType)
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
