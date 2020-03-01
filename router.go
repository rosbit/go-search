/**
 * REST API router
 * Rosbit Xu
 */
package main

import (
	"github.com/rosbit/http-helper"
	"net/http"
	"fmt"
	"go-search/conf"
	"go-search/rest"
	"go-search/indexer"
	"os"
	"log"
	"os/signal"
	"syscall"
)

// 设置路由，进入服务状态
func StartService() error {
	initIndexers()

	api := helper.NewHelper()

	api.GET("/schema/:index",    rest.ShowSchema)
	api.POST("/schema/:index",   rest.CreateSchema)
	api.DELETE("/schema/:index", rest.DeleteSchema)
	api.PUT("/schema/:index/:newIndex", rest.RenameSchema)
	api.PUT("/doc/:index",       rest.IndexDoc)
	api.PUT("/docs/:index",      rest.IndexDocs)
	api.PUT("/update/:index",    rest.UpdateDoc)
	api.DELETE("/doc/:index",    rest.DeleteDoc)
	api.DELETE("/docs/:index",   rest.DeleteDocs)
	api.GET("/search/:index",    rest.Search)

	// health check
	api.GET("/health", func(c *helper.Context) {
		c.String(http.StatusOK, "OK\n")
	})

	serviceConf := conf.ServiceConf
	listenParam := fmt.Sprintf("%s:%d", serviceConf.ListenHost, serviceConf.ListenPort)
	log.Printf("I am listening at %s...\n", listenParam)
	return http.ListenAndServe(listenParam, api)
}

func initIndexers() {
	indexer.StartIndexers(conf.ServiceConf.WorkerNum)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGSTOP, syscall.SIGQUIT)
	go func() {
		for range c {
			log.Println("I will exit in a while")
			indexer.StopIndexers(conf.ServiceConf.WorkerNum)
			os.Exit(0)
		}
	}()
}

