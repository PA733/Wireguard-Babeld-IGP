package static

import (
	"embed"
	"io/fs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var content embed.FS

// SPAFileSystem wraps the existing filesystem with SPA fallback
type SPAFileSystem struct {
	fs http.FileSystem
}

func (s SPAFileSystem) Open(name string) (http.File, error) {
	f, err := s.fs.Open(name)
	if os.IsNotExist(err) {
		return s.fs.Open("index.html")
	}
	return f, err
}

// GetFileSystem 返回嵌入的文件系统
func GetFileSystem() http.FileSystem {
	fsys, err := fs.Sub(content, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(fsys)
}

func Register(r *gin.Engine) {
	// 设置静态文件服务
	r.NoRoute(func(c *gin.Context) {
		spaFS := SPAFileSystem{fs: GetFileSystem()}
		fileServer := http.FileServer(spaFS)
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
