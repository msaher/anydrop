package main

import (
	"fmt"
	"net/http"
	"net"
	"os"
)

func myIp(interfaceName string) (*net.IPNet, error) {
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

func download(w http.ResponseWriter, r *http.Request) {
	fileName := "spaghetti.jpeg"
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

	attachment := fmt.Sprintf(`attachment; filename="%s"`, fileName)
	w.Header().Set("Content-Disposition", attachment)
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)

	return
}

func main() {
	// register handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/download", download)

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
