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
	"errors"
	"io/fs"
	"strings"
	"crypto/rand"
	"encoding/hex"
)

type App struct {
	downloadPath string
	uploadDir string
	template *template.Template
	token string
}

func (app *App) DownloadPath() (string, bool) {
	exists := app.downloadPath != ""
	return app.downloadPath, exists
}

func (app *App) download(w http.ResponseWriter, r *http.Request) {
	path, exists := app.DownloadPath()
	if !exists {
		http.Error(w, "No path to download", 400)
		return
	}

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

func (app *App) home(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]any)
	downloadPath, exists := app.DownloadPath()
	if exists {
		data["downloadBasename"] = filepath.Base(downloadPath)
	}
	uploadDir, err := collapseHome(app.uploadDir)
	if err != nil {
		uploadDir = app.uploadDir
	}
	data["uploadDir"] = uploadDir
	data["token"] = app.token

	err = app.template.Execute(w, data)
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

	dstPath, err := uniquePath(app.uploadDir, filepath.Base(header.Filename))
	if err != nil {
		http.Error(w, "failed to find a safe place to save", http.StatusInternalServerError)
		return
	}

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

func (app *App) withToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token != app.token {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	    next.ServeHTTP(w, r)
	    log.Printf("%s %s\n", r.Method, r.URL.Path)
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

func newToken(numBytes int) (string, error) {
    b := make([]byte, numBytes)
	_, err := rand.Read(b)
	if err != nil {
		return "", nil
	}
    return hex.EncodeToString(b), nil
}

// Changes /home/user/foo to ~/foo
func collapseHome(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, home) {
		return path, nil
	}

	collapsed := "~" + path[len(home):]
	return collapsed, nil
}

func uniquePath(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	ext := filepath.Ext(path)
	noext := name[:len(name)-len(ext)]

	i := 1
	for {
		_ , err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
		if err != nil {
			return "", err
		}
		base := fmt.Sprintf("%s_%d%s", noext, i, ext)
		path = filepath.Join(dir, base)
		i++
	}
}

func isDirWritable(path string) error {
	file, err := os.CreateTemp(path, ".permcheck")
	if err != nil {
		return err
	}
	file.Close()
	os.Remove(file.Name())

	return nil
}

func isFileAccessible(path string) error {
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
	app.template = template.Must(template.New("home").ParseFiles("ui/home.html"))

	token, err := newToken(8)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't create token: %s\n", err)
		return
	}
	app.token = token

	flag.StringVar(&app.downloadPath, "download", "", "file to download on /download")
	flag.StringVar(&app.uploadDir, "upload-dir", "", "upload directory (default cwd)")
	port := flag.Int("port", 8000, "port to listen to")

	flag.Parse()

	// check download file is accessible
	downloadPath, exists := app.DownloadPath()
	if exists {
		err := isFileAccessible(downloadPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "download file: %s", err)
			return
		}
		// try to open it just incase
		file, err := os.Open(downloadPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "download file: %s", err)
			return
		}
		file.Close()
	}

	// default to home directory
	if app.uploadDir == "" {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get current working directory: %s\n", err)
			return
		}
		app.uploadDir = dir
	}

	// BUG: we don't bother checking this in the http handler. Lazy.
	// check upload directory is valid
	info, err := os.Stat(app.uploadDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to access directory: %s\n", err)
		return
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Not a directory: %s\n", app.uploadDir)
		return
	}
	if err := isDirWritable(app.uploadDir); err != nil {
		fmt.Fprintf(os.Stderr, "directory is not writable: %s\n", err)
		return
	}

	// register handlers
	var handler http.Handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", app.home)
	mux.HandleFunc("/download", app.download)
	mux.HandleFunc("POST /upload", app.postUpload)
	handler = app.withToken(mux)
	handler = logHandler(handler)

	log.SetFlags(0) // dont want dates in logger

	addr := fmt.Sprintf(":%d", *port)
	server := http.Server {
		Addr: addr,
		Handler: handler,
	}

	ip, err := myIp()
	if err != nil {
		panic(err)
	}
	ipv4 := ip.IP.To4()
	url := fmt.Sprintf("http://%s%s?token=%s", ipv4, server.Addr, app.token)
	fmt.Printf("Listening on %s\n", url)

	qr, err := qrcode.New(url, qrcode.Low)
	if err != nil {
		// show error but dont exit.
		fmt.Fprintf(os.Stderr, "Failed to create qrcode: %s", err)
	} else {
		fmt.Print(qr.ToSmallString(true))
	}

	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
