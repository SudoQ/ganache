package service

import (
	"errors"
	"fmt"
	"github.com/SudoQ/crisp/item"
	"github.com/gorilla/mux"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Service struct {
	URL    string
	Port   string
	Period time.Duration
	Limit  uint
	Cache  *item.Item
	logger *log.Logger
}

func New(targetUrl, port string, limit uint) *Service {
	period, err := LimitToPeriod(limit)
	if err != nil {
		limit = 1
		period, _ = LimitToPeriod(limit)
	}
	srv := &Service{
		URL:    targetUrl,
		Port:   port,
		Period: period,
		Limit:  limit,
		Cache:  nil,
		logger: nil,
	}
	srv.initLogger()
	return srv
}

func (this *Service) initLogger() {
	u, err := url.Parse(this.URL)
	label := u.Host
	if err != nil {
		label = "?"
	}
	this.logger = log.New(os.Stdin, fmt.Sprintf("crisp[%s]: ", label), log.Lshortfile)
}

func LimitToPeriod(limit uint) (time.Duration, error) {
	if limit == 0 {
		return 0, errors.New("Division with zero")
	}

	period := (60.0 / float64(limit)) * 60
	return (time.Duration(period) * time.Second), nil
}

func (this *Service) Collect() {
	for {
		func() {
			now := time.Now()
			timeDelta := now.Sub(this.Cache.Timestamp)
			if this.Cache.Timestamp.Before(now) && timeDelta > this.Period {
				this.logger.Printf("GET %s\n", this.URL)

				resp, err := http.Get(this.URL)
				if err != nil {
					this.logger.Println(err)
					return
				}

				payload, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					this.logger.Println(err)
					return
				}

				newItem := item.New(now, payload)
				this.Cache = newItem
				cacheFilename := "cache.json"
				err = this.Cache.WriteFile(cacheFilename)
				if err != nil {
					this.logger.Fatal(err)
				}
				this.logger.Printf("Saved cache to %s\n", cacheFilename)
			}
		}()
		select {
		case <-time.After(this.Period):
			continue
		}
	}
}

func (this *Service) LoadCache(filename string) error {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	newItem, err := item.NewFromJSON(content)
	if err != nil {
		return err
	}
	this.Cache = newItem
	return nil
}

func (this *Service) Info() string {
	return fmt.Sprintf("Crisp API caching service v0.1")
}

func (this *Service) Run() {
	err := this.LoadCache("cache.json")
	if err != nil {
		this.logger.Fatal(err)
	}
	go this.Collect()
	r := mux.NewRouter()
	r.HandleFunc("/", this.HomeHandler)
	r.HandleFunc("/info", this.InfoHandler)
	r.HandleFunc("/cache.json", this.CacheHandler)
	port := fmt.Sprintf(":%s", this.Port)
	http.ListenAndServe(port, r)
}

func (this *Service) HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(this.Cache.Payload)
}

func (this *Service) InfoHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte(this.Info()))
}

func (this *Service) CacheHandler(w http.ResponseWriter, r *http.Request) {
	response, err := this.Cache.JSON()
	if err != nil {
		this.logger.Println(err)
		w.WriteHeader(404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(response)
}
