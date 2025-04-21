// $ go build && ./share
// $ go install
// $ go run %f
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt" // Println
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
    "path/filepath"
)

//go:embed index.html
var index []byte

const (
	KB      = 1024
	MB      = KB * KB
	GB      = MB * KB
	bufSize = 64 * KB
)

func ReadableSize(length int64) string {
	var value = float64(length)
	var unit string
	switch {
	case value >= GB:
		value /= GB
		unit = "GB"
	case value >= MB:
		value /= MB
		unit = "MB"
	case value >= KB:
		value /= KB
		unit = "KB"
	default:
		unit = "B"
	}
	return fmt.Sprintf("%.2f%s", value, unit)
}

func getFilename(name string) string {
    // add number if another file with same name exists
    if _, err := os.Stat(name); os.IsNotExist(err) {
        return name
    }
    ext := filepath.Ext(name)
    base := string(name[:len(name)-len(ext)])
    for i := 1; ; i++ {
        newName := fmt.Sprintf("%s (%d)%s", base, i, ext)
        if _, err := os.Stat(newName); os.IsNotExist(err) {
            return newName
        }
    }
}

type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isdir"`
	Size  string `json:"size"`
}

type handler struct{}

func (handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	path := req.URL.Path[1:]
	if path == "" {
		path = "."
	}
	if req.Method == "GET" || req.Method == "HEAD" {
		// log.info(req.Method, path)
		if req.URL.Path == "/" && req.Header.Get("Referer") == "" {
			res.Write(index)
			return
		}
		if path == "" {
			path = "."
		}
		f, err := os.Open(path)
		if err != nil {
			res.WriteHeader(http.StatusNotFound)
			log.Print(http.StatusNotFound, err)
			return
		}
		defer f.Close()
		s, err := f.Stat()
		if err != nil {
			res.WriteHeader(http.StatusExpectationFailed)
			log.Print(http.StatusExpectationFailed, err)
			return
		}
		if s.IsDir() {
			entries, err := f.ReadDir(0)
			if err != nil {
				res.WriteHeader(http.StatusExpectationFailed)
				log.Print(http.StatusExpectationFailed, err)
				return
			}
			jentries := make([]DirEntry, 0)
			for _, entry := range entries {
				jentry := DirEntry{entry.Name(), entry.IsDir(), ""}
				if !entry.IsDir() {
					// file
					info, err := entry.Info()
					if err != nil {
						res.WriteHeader(http.StatusExpectationFailed)
						log.Print(http.StatusExpectationFailed, err)
						return
					}
					jentry.Size = ReadableSize(info.Size())
				}
				jentries = append(jentries, jentry)
			}
			jsonData, err := json.Marshal(jentries)
			if err != nil {
				res.WriteHeader(http.StatusExpectationFailed)
				log.Print(http.StatusExpectationFailed, err)
				return
			}
			res.Write(jsonData)
		} else {
			http.ServeContent(res, req, path, s.ModTime(), f)
		}
	} else if req.Method == "POST" {
		req.ParseMultipartForm(24 * 1024 * 1024 * 1024)
		for _, file := range req.MultipartForm.File["file"] {
			w, err := os.Create(getFilename(filepath.Join(path, file.Filename)))
			if err != nil {
				res.WriteHeader(http.StatusExpectationFailed)
				log.Print(http.StatusExpectationFailed, err)
				return
			}
			defer w.Close()
			r, err := file.Open()
			if err != nil {
				res.WriteHeader(http.StatusExpectationFailed)
				log.Print(http.StatusExpectationFailed, err)
				return
			}
			defer r.Close()
			io.Copy(w, r)
		}
        res.WriteHeader(http.StatusOK)
	}
}

func showIP(port string) {
	interfaces, _ := net.Interfaces()
	workingInterfaces := make([]net.Interface, 0)
	fmt.Println("Serving on:\n  localhost (Loop back)")
	for _, i := range interfaces {
		if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagLoopback == 0 {
			workingInterfaces = append(workingInterfaces, i)
		}
	}
	if len(workingInterfaces) == 0 {
		return
	}
	for _, i := range workingInterfaces {
		addrs, err := i.Addrs()
		fmt.Print("  ")
		if err != nil {
			log.Fatal("Network error")
			return
		}
		for _, a := range addrs {
			var ip string
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP.String()
			case *net.IPAddr:
				ip = "[" + v.IP.String() + "]"
			}
			fmt.Print(ip, ":", port, "  ")
		}
		fmt.Println("(" + i.Name + ")")
	}
}

func main() {
	port := "5999"
	var srv http.Server = http.Server{
		Handler: handler{},
		Addr: "0.0.0.0:" + port,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			log.Printf("HTTP server Shutdown: %v", err)
		} else {
			log.Printf("HTTP server Shutdown")
		}
		close(idleConnsClosed)
	}()
	showIP(port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	<-idleConnsClosed
}
