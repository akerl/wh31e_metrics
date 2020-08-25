package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/ghodss/yaml"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
)

type sensorData struct {
	TimeStr      string  `json:"time"`
	Id           int     `json:"id"`
	Channel      int     `json:"channel"`
	BatteryInt   int     `json:"battery_ok"`
	TemperatureC float64 `json:"temperature_C"`
	Humidity     int     `json:"humidity"`
}

func (s sensorData) TemperatureF() float64 {
	return s.TemperatureC*9/5 + 32
}

func (s sensorData) Time() time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s.TimeStr)
	if err != nil {
		panic(fmt.Sprintf("time parse failed: %s", s.TimeStr))
	}
	return t
}

type config struct {
	APIKey    string         `json:"apikey"`
	Names     map[int]string `json:"names"`
	harvester *telemetry.Harvester
}

func LoadConfig(file string) *config {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		panic(fmt.Sprintf("config read failed: %s", err))
	}
	var c config
	err = yaml.Unmarshal(contents, &c)
	if err != nil {
		panic(fmt.Sprintf("config parse failed: %s", err))
	}
	return &c
}

func (c *config) Harvester() *telemetry.Harvester {
	if c.harvester != nil {
		return c.harvester
	}
	h, err := telemetry.NewHarvester(
		telemetry.ConfigAPIKey(c.APIKey),
		telemetry.ConfigBasicErrorLogger(os.Stdout),
	)
	if err != nil {
		panic(fmt.Sprintf("harvester failed: %s", err))
	}
	c.harvester = h
	return h
}

func (c *config) GetName(id int) string {
	name := c.Names[id]
	if name == "" {
		name = string(id)
	}
	return name
}

func (c *config) Parse(raw []byte) (sensorData, error) {
	var data sensorData
	err := json.Unmarshal(raw, &data)
	return data, err
}

func (c *config) Submit(data sensorData) error {
	ts := data.Time()
	attrs := map[string]interface{}{
		"channel": data.Channel,
		"id":      data.Id,
		"name":    c.GetName(data.Id),
	}

	c.Harvester().RecordMetric(telemetry.Gauge{
		Name:       "temperature",
		Value:      data.TemperatureF(),
		Timestamp:  ts,
		Attributes: attrs,
	})

	c.Harvester().RecordMetric(telemetry.Gauge{
		Name:       "humidity",
		Value:      float64(data.Humidity),
		Timestamp:  ts,
		Attributes: attrs,
	})

	c.Harvester().RecordMetric(telemetry.Gauge{
		Name:       "battery",
		Value:      float64(data.BatteryInt),
		Timestamp:  ts,
		Attributes: attrs,
	})
	return nil
}

func (c *config) Push() {
	c.Harvester().HarvestNow(context.Background())
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("no config file provided")
		os.Exit(1)
	}
	c := LoadConfig(os.Args[1])

	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadBytes('\n')

		if err != nil {
			if err == io.EOF {
				return
			}
			panic(fmt.Sprintf("reader error: %s", err))
		}

		data, err := c.Parse(line)
		if err != nil {
			panic(fmt.Sprintf("parser error: %s / %s", line, err))
		}

		err = c.Submit(data)
		if err != nil {
			panic(fmt.Sprintf("submit error: %s / %s", line, err))
		}
	}

	c.Push()
}
