// -{go install}
// -{go build && share}
// -{go run %f}
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
)

//go:embed index.html
var index []byte

const (
	KB = 1024
	MB = KB * KB
	GB = MB * KB
	// bufSize = 64 * KB
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

type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isdir"`
	Size  string  `json:"size"`
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
			res.WriteHeader(http.StatusOK)
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
			res.WriteHeader(http.StatusOK)
			res.Write(jsonData)
		} else {
			http.ServeContent(res, req, path, s.ModTime(), f)
		}
	} else if req.Method == "POST" {
		req.ParseMultipartForm(24 * 1024 * 1024 * 1024)
		// fmt.Println(req.MultipartForm.Value["folder"][0], req.MultipartForm.File["file"])
		res.WriteHeader(http.StatusOK)
		for _, file := range req.MultipartForm.File["file"] {
			w, err := os.Create(path + "/" + file.Filename)
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
			// var written int64 = 0
			// for file.Size - written > bufSize {
			//     wrote, err := io.CopyN(w, r, bufSize)
			//     if err != nil && err != io.EOF {
			//         res.WriteHeader(http.StatusExpectationFailed)
			//         log.Print(http.StatusExpectationFailed, err)
			//         return
			//     }
			//     written += wrote
			//     // res.Write([]byte(fmt.Sprint(written * 100 / file.Size)))
			//     fmt.Println(written * 100 / file.Size)
			// }
			// _, err = io.CopyN(w, r, bufSize - (file.Size - written))
			// if err != nil && err != io.EOF {
			//     res.WriteHeader(http.StatusExpectationFailed)
			//     log.Print(http.StatusExpectationFailed, err)
			//     return
			// }
		}
	}
}

func main() {
	var srv http.Server = http.Server{
		Handler: handler{},
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
    // show IP address
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Serving on", conn.LocalAddr().(*net.UDPAddr).IP)  // without port, using 80
    conn.Close()
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	<-idleConnsClosed
}
