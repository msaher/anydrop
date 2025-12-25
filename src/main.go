package main

import (
	"fmt"
	"net/http"
	"html/template"
	"net"
	"os"
	"io"
	"flag"
	"path/filepath"
	"github.com/skip2/go-qrcode"
	"log"
)

type App struct {
	downloadPath string
	uploadTemplate *template.Template
}

func (app *App) download(w http.ResponseWriter, r *http.Request) {
	path := app.downloadPath

	stat, err := os.Stat(path)
	if err != nil {
		http.Error(w, "stat failed", 500)
		return
	}
	if stat.IsDir() {
		http.Error(w, "somehow got a directory", 500)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		http.Error(w, "can't open file", 500)
		return
	}
	defer file.Close()

	// no cache. We may use the same url but we may have
	// different files
	w.Header().Set("Cache-Control", "no-store")

	fileName := filepath.Base(path)
	attachment := fmt.Sprintf(`attachment; filename="%s"`, fileName)
	w.Header().Set("Content-Disposition", attachment)
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)

	return
}

func (app *App) getUpload(w http.ResponseWriter, r *http.Request) {
	err := app.uploadTemplate.Execute(w, nil)
	if err != nil {
		panic(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

func (app *App) postUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	uploadDir := "anydrop"
	err = os.MkdirAll(uploadDir, 0755)
	if err != nil {
		http.Error(w, "failed to create upload dir", http.StatusInternalServerError)
		return
	}

	dstPath := filepath.Join(uploadDir, filepath.Base(header.Filename))
	// TODO: dont overwrite file immediately. Make sure it does not exist already,
	// and if it does append timestamp or something
	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File uploaded: %s\n", header.Filename)
	log.Printf("Uploaded file: %s from %s", header.Filename, r.RemoteAddr)
}

func logHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
	    next.ServeHTTP(writer, req)
	    log.Printf("%s %s\n", req.Method, req.URL.Path)
	})
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

	// check template is fine
	app.uploadTemplate = template.Must(template.New("upload").ParseFiles("ui/upload.html"))

	flag.StringVar(&app.downloadPath, "download", "", "file to download on /download")
	flag.Parse()

	if app.downloadPath == "" {
		fmt.Fprintf(os.Stderr, "must specify a -download path")
		return
	}

	// check file is accessiable
	err := isPathAccessible(app.downloadPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download file: %s", err)
		return
	}
	// try to open it just incase
	file, err := os.Open(app.downloadPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download file: %s", err)
		return
	}
	file.Close()

	// register handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/download", app.download)
	mux.HandleFunc("GET /upload", app.getUpload)
	mux.HandleFunc("POST /upload", app.postUpload)
	handler := logHandler(mux)

	log.SetFlags(0) // dont want dates in logger


	addr := ":8000"
	server := http.Server {
		Addr: addr,
		Handler: handler,
	}

	ip, err := myIp()
	if err != nil {
		panic(err)
	}
	ipv4 := ip.IP.To4()
	url := fmt.Sprintf("http://%s%s", ipv4, server.Addr)

	urlUpload := fmt.Sprintf("%s/upload", url)
	qr, err := qrcode.New(urlUpload, qrcode.Low)
	if err != nil {
		// show error but dont exit.
		fmt.Fprintf(os.Stderr, "Failed to create qrcode: %s", err)
	} else {
		fmt.Print("Upload QR:")
		fmt.Print(qr.ToSmallString(true))
	}

	urlDownload := fmt.Sprintf("%s/download", url)
	qr, err = qrcode.New(urlDownload, qrcode.Low)
	if err != nil {
		// show error but dont exit.
		fmt.Fprintf(os.Stderr, "Failed to create qrcode: %s", err)
	} else {
		fmt.Print("Download QR:")
		fmt.Print(qr.ToSmallString(true))
	}

	fmt.Printf("Listening on %s\n", url)
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
