package main

import (
	"fmt"
	"github.com/QQGoblin/go-sdk/example/toy_web/http"
	"io"
)

type TestObj struct {
	Message   string
	OriginURL string
}

func Hello(c *http.Context) {
	c.StatusOK(TestObj{
		Message:   "Hello",
		OriginURL: c.R.URL.Path,
	})
}

func Say(c *http.Context) {

	name := c.Params["id"]

	body, err := io.ReadAll(c.R.Body)
	if err != nil {
		c.W.WriteHeader(500)
		return
	}

	c.StatusOK(TestObj{
		Message:   fmt.Sprintf("%s say: %s", name, string(body)),
		OriginURL: c.R.URL.Path,
	})
}

func main() {

	gracefulShutdown := http.NewGracefulShutdown()
	s := http.NewHttpServerWithFilter("http-server", http.MetricsFilterBuilder, gracefulShutdown.ShutdownFilterBuilder)
	s.Register("GET", "/", Hello)
	s.Register("POST", "/:id/say", Say)
	go func() {
		if err := s.Start(":8080"); err != nil {
			panic(err)
		}
	}()
	http.WaitForShutdown(
		gracefulShutdown.RejectNewRequestAndWaiting,
	)
}
