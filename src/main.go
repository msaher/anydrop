package main

import (
	"fmt"
	"net/http"
	"net"
	"os"
	"flag"
)

type App struct {
	downloadFileName string
}

func (app *App) download(w http.ResponseWriter, r *http.Request) {
	fileName := app.downloadFileName

	stat, err := os.Stat(fileName)
	if err != nil {
		http.Error(w, "stat failed", 500)
		return
	}
	if stat.IsDir() {
		http.Error(w, "somehow got a directory", 500)
		return
	}

	file, err := os.Open(fileName)
	if err != nil {
		http.Error(w, "can't open file", 500)
		return
	}
	defer file.Close()

	// no cache. We may use the same url but we may have
	// different files
	w.Header().Set("Cache-Control", "no-store")

	attachment := fmt.Sprintf(`attachment; filename="%s"`, fileName)
	w.Header().Set("Content-Disposition", attachment)
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)

	return
}

func myIp() (*net.IPNet, error) {
	// get first non-loopback address
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var iface *net.Interface
	for _, netInterface := range ifaces {
		if netInterface.Name != "lo" {
			iface = &netInterface
			break
		}
	}
	// not found
	if iface == nil {
		return nil, fmt.Errorf("Interface not found")
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP.To4()
		if ip != nil {
			return ipNet, nil
		}
	}

	return nil, fmt.Errorf("couldn't find an Ipv4 address")
}

func isPathAccessible(path string) error {
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return fmt.Errorf("`%s` is a directory", path)
	}
	return nil
}

func main() {
	var app App

	flag.StringVar(&app.downloadFileName, "download", "", "file to download on /download")
	flag.Parse()

	if app.downloadFileName == "" {
		fmt.Fprintf(os.Stderr, "must specify a -download path")
		return
	}

	// check file is accessiable
	err := isPathAccessible(app.downloadFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download file: %s", err)
		return
	}
	// try to open it just incase
	file, err := os.Open(app.downloadFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download file: %s", err)
		return
	}
	file.Close()

	// register handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/download", app.download)

	addr := ":8000"
	server := http.Server {
		Addr: addr,
		Handler: mux,
	}

	ip, err := myIp()
	if err != nil {
		panic(err)
	}
	ipv4 := ip.IP.To4()
	fmt.Printf("Listening on http://%s%s\n", ipv4, server.Addr)
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
