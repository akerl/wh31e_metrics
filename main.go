package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/ghodss/yaml"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

// Version is overridden by link flags during build
var Version = "unset"

type config struct {
	InfluxURL    string         `json:"influx_url"`
	InfluxToken  string         `json:"influx_token"`
	InfluxOrg    string         `json:"influx_org"`
	InfluxBucket string         `json:"influx_bucket"`
	SyslogHost   string         `json:"syslog_host"`
	SyslogPort   int            `json:"syslog_port"`
	SensorNames  map[int]string `json:"sensor_names"`
}

type message struct {
	TimeStr      string  `json:"time"`
	ID           int     `json:"id"`
	Channel      int     `json:"channel"`
	BatteryInt   int     `json:"battery_ok"`
	TemperatureC float64 `json:"temperature_C"`
	Humidity     int     `json:"humidity"`
}

func (m message) ChannelStr() string {
	return fmt.Sprintf("%d", m.Channel)
}

func (m message) IDStr() string {
	return fmt.Sprintf("%d", m.ID)
}

func (m message) Name(conf config) string {
	name := conf.SensorNames[m.Channel]
	if name == "" {
		name = fmt.Sprintf("%d", m.Channel)
	}
	return name
}

func (m message) TemperatureF() float64 {
	return m.TemperatureC*9/5 + 32
}

func (m message) Time() (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", m.TimeStr)
}

func (m message) ToPoint(conf config) (*write.Point, error) {
	t, err := m.Time()
	if err != nil {
		return nil, err
	}
	p := influxdb2.NewPoint(
		"wh31e",
		map[string]string{
			"channel": m.ChannelStr(),
			"id":      m.IDStr(),
			"name":    m.Name(conf),
		},
		map[string]interface{}{
			"temperature_f": m.TemperatureF(),
			"temperature_c": m.TemperatureC,
			"humidity":      m.Humidity,
			"battery":       m.BatteryInt,
		},
		t,
	)
	return p, nil
}

func parse(log format.LogParts) (message, error) {
	var m message

	data, ok := log["message"].(string)
	if !ok {
		return m, fmt.Errorf("failed to coerce message to byte slice")
	}

	err := json.Unmarshal([]byte(data), &m)
	return m, err
}

func loop(conf config, channel syslog.LogPartsChannel) error {
	client := influxdb2.NewClient(conf.InfluxURL, conf.InfluxToken)
	defer client.Close()
	writeAPI := client.WriteAPIBlocking(conf.InfluxOrg, conf.InfluxBucket)

	for log := range channel {
		m, err := parse(log)
		if err != nil {
			return err
		}
		p, err := m.ToPoint(conf)
		if err != nil {
			return err
		}
		err = writeAPI.WritePoint(context.Background(), p)
		if err != nil {
			return err
		}
	}
	return nil
}

func launchSyslogServer(conf config, channel syslog.LogPartsChannel) error {
	handler := syslog.NewChannelHandler(channel)
	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	host := fmt.Sprintf("%s:%d", conf.SyslogHost, conf.SyslogPort)
	server.ListenUDP(host)
	return server.Boot()
}

func loadConfig(file string) (config, error) {
	var c config

	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return c, err
	}

	err = yaml.Unmarshal(contents, &c)
	return c, err
}

func start() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("config file must be given as argument")
	}
	c, err := loadConfig(os.Args[1])
	if err != nil {
		return err
	}
	channel := make(syslog.LogPartsChannel)
	if err := launchSyslogServer(c, channel); err != nil {
		return err
	}
	if err := loop(c, channel); err != nil {
		return err
	}
	return nil
}

func main() {
	err := start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
