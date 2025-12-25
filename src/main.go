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
	file, err := os.Open(fileName)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		msg := fmt.Sprintf("not found: %s", fileName)
		http.Error(w, msg, 404)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "stat failed", 500)
		return
	}

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


func main() {
	var app App

	flag.StringVar(&app.downloadFileName, "download", "", "file to download on /download")
	flag.Parse()

	if app.downloadFileName == "" {
		fmt.Fprintf(os.Stderr, "must specify a -download path")
	}

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
