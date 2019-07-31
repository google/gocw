// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gocw"
	"github.com/google/gocw/util"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
	"github.com/labstack/echo"
)

var (
	portFlag = flag.Int("port", 8080, "Server HTTP port number")
	dirFlag  = flag.String("dir", "captures", "Input captures directory to display")
)

const (
	capExt = ".json.gz"
)

type TraceMetadata struct {
	Id         int    `json:"Id"`
	Key        string `json:"Key"`
	Pt         string `json:"PT"`
	Ct         string `json:"CT"`
	NumSamples int    `json:"NumSamples"`
}

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

func capturesDirectory() string {
	return path.Join(projectRoot(), *dirFlag)
}

// A go-routine that waits for directory changes.
// Notifies changes by publishing a message via broker.
func watchDirectoryChanges(broker *util.Broker) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Errorf("NewWatcher failed: %v", err)
		return
	}
	defer watcher.Close()

	err = watcher.Add(capturesDirectory())
	if err != nil {
		glog.Errorf("watcher.Add failed: %v", err)
		return
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				glog.Warning("watcher.Events is not ok. Aborting")
				return
			}
			glog.V(1).Infof("Watcher event:", event)
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create ||
				event.Op&fsnotify.Remove == fsnotify.Remove ||
				event.Op&fsnotify.Rename == fsnotify.Rename {
				if strings.HasSuffix(event.Name, capExt) {
					broker.Publish(event)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				glog.Warning("watcher.Errors is not ok. Aborting")
				return
			}
			glog.Warning("Watcher error:", err)
		}
	}
}

func waitForCaptures(c echo.Context, watcher *util.Broker) error {
	var wg sync.WaitGroup
	timedOut := time.NewTimer(5 * time.Minute)

	wg.Add(1)
	go func() {
		defer wg.Done()
		dirChanged := watcher.Subscribe()
		defer watcher.Unsubscribe(dirChanged)

		for {
			select {
			case <-timedOut.C:
				glog.V(1).Infof("Timed out")
				return
			case <-c.Request().Context().Done():
				glog.V(1).Infof("Client disconnected")
				return
			case <-dirChanged:
				glog.V(1).Infof("Received dir notification from broker")
				return
			}
		}
	}()

	wg.Wait()
	return nil
}

func loadCapture(filename string) (gocw.Capture, error) {
	return gocw.LoadCapture(path.Join(capturesDirectory(), filename+capExt))
}

func main() {
	defer glog.Flush()

	watchBroker := util.NewBroker()
	go watchBroker.Start()
	go watchDirectoryChanges(watchBroker)

	e := echo.New()

	// Static files.
	e.File("/", "viewer/index.html")
	e.File("/viewer.js", "viewer/viewer.js")
	e.File("/viewer.css", "viewer/viewer.css")

	// Returns list of capture files in directory.
	e.GET("/captures", func(c echo.Context) error {
		if c.QueryParam("wait") != "false" {
			waitForCaptures(c, watchBroker)
		}
		files, err := filepath.Glob(path.Join(capturesDirectory(), "*"+capExt))
		if err != nil {
			glog.Errorf("Glob failed: %v", err)
			return err
		}
		for i, f := range files {
			files[i] = strings.TrimSuffix(filepath.Base(f), capExt)
		}
		return c.JSON(http.StatusOK, files)
	})

	// Returns trace data from a single capture file.
	e.GET("/data/:capture", func(c echo.Context) error {
		capture, err := loadCapture(c.Param("capture"))
		if err != nil {
			glog.Errorf("Error loading capture file: %v", err)
			return err
		}
		var metadata []TraceMetadata
		for i, t := range capture {
			metadata = append(metadata, TraceMetadata{i,
				hex.EncodeToString(t.Key),
				hex.EncodeToString(t.Pt),
				hex.EncodeToString(t.Ct),
				len(t.PowerMeasurements)})
		}
		return c.JSON(http.StatusOK, metadata)
	})
	e.GET("/data/:capture/:trace", func(c echo.Context) error {
		capture, err := loadCapture(c.Param("capture"))
		if err != nil {
			glog.Errorf("Error loading capture file: %v", err)
			return err
		}
		trace, err := strconv.Atoi(c.Param("trace"))
		if err != nil || trace < 0 || trace >= len(capture) {
			return c.String(http.StatusInternalServerError, "Invalid trace")

		}
		return c.JSON(http.StatusOK, capture[trace].PowerMeasurements)
	})

	glog.Fatal(e.Start(fmt.Sprintf(":%d", *portFlag)))
}
