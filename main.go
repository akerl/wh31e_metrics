package main

import (
	"encoding/json"
	"fmt"

	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

type message struct {
	TimeStr      string  `json:"time"`
	ID           int     `json:"id"`
	Channel      int     `json:"channel"`
	BatteryInt   int     `json:"battery_ok"`
	TemperatureC float64 `json:"temperature_C"`
	Humidity     int     `json:"humidity"`
}

func handle(log format.LogParts) error {
	data, ok := log["message"].([]byte)
	if !ok {
		return fmt.Errorf("failed to coerce message to byte slice")
	}

	var m message
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", m)
	return nil
}

func loop(channel syslog.LogPartsChannel) {
	for msg := range channel {
		if err := handle(msg); err != nil {
			panic(err)
		}
	}
}

func main() {
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)
	go loop(channel)

	server := syslog.NewServer()
	server.SetFormat(syslog.RFC5424)
	server.SetHandler(handler)
	server.ListenUDP("127.0.0.1:514")

	server.Boot()
	server.Wait()
}
