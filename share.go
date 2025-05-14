// $ go build && ./share
// $ go install
// $ go run %f
package main

import (
	"context"
	_ "embed"
	"fmt" // Println
	"html/template"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	KB      = 1024
	MB      = KB * KB
	GB      = MB * KB
	bufSize = 64 * KB
)

//go:embed nodes.gotmpl
var templ []byte

var imageExts = []string{"jpg", "jpeg", "png", "jfif"}

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
	IsImage bool `json:"isimage"`
	Size  string `json:"size"`
}

type ResData struct {
	Name string
	Path string
	IsImage bool
	Prev string
	Next string
	Entries []DirEntry
}

func checkImage(name string) bool {
	for _, ext := range imageExts {
		if strings.HasSuffix(name, "." + ext) {
			return true
		}
	}
	return false
}

func splitTail(path string) (string, string) {
	iName := strings.LastIndex(path, "/")
	if iName == -1 {
		return "", path
	}
	return path[:iName], path[iName+1:]
}

type Handler struct{
	templ *template.Template
	basePathMask string
	basePath string
}

func (self *Handler) Init() {
	if len(os.Args) > 1 {
		self.basePath = os.Args[1]
		self.basePathMask = "/" + strconv.Itoa(rand.Int()) // more secure
	}
	tmpl, err := template.New("Index").Parse(string(templ))
	if err != nil {
		panic(err)
	}
	self.templ = tmpl
}

func (self *Handler) ServeT(res io.Writer, data any) {
	err := self.templ.Execute(res, data)
	if err != nil {
		panic(err)
	}
}

func (self *Handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if !strings.HasPrefix(path, self.basePathMask) {
		res.WriteHeader(http.StatusNotFound)
		log.Print(http.StatusNotFound, "Failed base path check: " + path)
		return
	}
	path = path[len(self.basePathMask):]
	path = strings.Trim(path, "/")
	showPath := path
	if showPath == "" {
		showPath = "."
	}
	if self.basePath != "" {
		path = self.basePath + "/" + path
	}
	if path == "" {
		path = "."
	}
	if req.Method == "POST" {
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
		http.Redirect(res, req, self.basePathMask + "/" + path, http.StatusSeeOther)
		return
	}
	parent, name := splitTail(path)
	viewImage := checkImage(path) && req.URL.Query().Get("view") != ""
	openPath := path
	if viewImage {
		openPath = parent
	}
	f, err := os.Open(openPath)
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
	data := ResData{
		Name: name,
		Path: showPath,
		IsImage: viewImage,
	}
	if viewImage {
		// view image
		// get neighbors
		entries, err := f.ReadDir(0)
		if err != nil {
			res.WriteHeader(http.StatusExpectationFailed)
			log.Print(http.StatusExpectationFailed, err)
			return
		}
		iImg := -1
		for i, entry := range entries {
			if !checkImage(entry.Name()) {
				continue
			}
			if entry.Name() == name {
				iImg = i
				continue
			}
			if iImg == -1 {
				data.Prev = entry.Name()
			} else {
				data.Next = entry.Name()
				break
			}
		}
		if iImg == -1 {
			res.WriteHeader(http.StatusExpectationFailed)
			log.Print(http.StatusExpectationFailed, "Image not found")
			return
		}
		self.ServeT(res, data)
		return
	}
	if !s.IsDir() {
		// simple serve file
		http.ServeContent(res, req, path, s.ModTime(), f)
		return
	}
	// dir list
	entries, err := f.ReadDir(0)
	if err != nil {
		res.WriteHeader(http.StatusExpectationFailed)
		log.Print(http.StatusExpectationFailed, err)
		return
	}
	for _, entry := range entries {
		lentry := DirEntry{Name: entry.Name(), IsDir: entry.IsDir(), IsImage: checkImage(entry.Name())}
		if !entry.IsDir() {
			// file
			info, err := entry.Info()
			if err != nil {
				res.WriteHeader(http.StatusExpectationFailed)
				log.Print(http.StatusExpectationFailed, err)
				return
			}
			lentry.Size = ReadableSize(info.Size())
		}
		data.Entries = append(data.Entries, lentry)
	}
	self.ServeT(res, data)
	return
}

func showIP(port string, basePath string) {
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
				if v.IP.To4() == nil {
					ip = "[" + v.IP.String() + "]"
				} else {
					ip = v.IP.String()
				}
			}
			fmt.Printf("http://%s:%s%s/ ", ip, port, basePath)
		}
		fmt.Println("(" + i.Name + ")")
	}
}

func main() {
	port := "5999"
	handler := Handler{}
	handler.Init()
	var srv http.Server = http.Server{
		Handler: &handler,
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
	showIP(port, handler.basePathMask)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	<-idleConnsClosed
}
