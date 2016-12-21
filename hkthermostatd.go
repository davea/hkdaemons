package main

import (
	"encoding/json"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
	"io/ioutil"
	"log"
	"strconv"
)

func main() {

	file, err := ioutil.ReadFile("./hkthermostatd.json")
	if err != nil {
		panic(err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(file, &config); err != nil {
		panic(err)
	}

	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			log.Println(err)
		},
	})

	defer cli.Terminate()

	err = cli.Connect(&client.ConnectOptions{
		Network:  "tcp",
		Address:  config["broker"].(string),
		ClientID: []byte(config["client_id"].(string)),
	})
	if err != nil {
		panic(err)
	}

	var accessories []*accessory.Accessory

	configArr, ok := config["accessories"].([]interface{})
	if !ok {
		panic(err)
	}

	for _, accessory_config := range configArr {
		accessory_config := accessory_config.(map[string]interface{})
		info := accessory.Info{
			Name:         accessory_config["name"].(string),
			Manufacturer: config["manufacturer"].(string),
		}
		state_topic := fmt.Sprintf("sensor/temperature/%s", accessory_config["id"].(string))
		target_topic := fmt.Sprintf("sensor/temperature/%s/target", accessory_config["id"].(string))
		active_topic := fmt.Sprintf("sensor/temperature/%s/active", accessory_config["id"].(string))

		is_thermostat := accessory_config["controllable"].(bool)
		if is_thermostat {
			acc := accessory.NewThermostat(info, 0, -5, 30, 0.1)

			acc.Thermostat.TargetTemperature.OnValueRemoteUpdate(func(value float64) {
				log.Printf("%s TargetTemperature: %02f", info.Name, value)
				err = cli.Publish(&client.PublishOptions{
					QoS:       mqtt.QoS0,
					Retain:    true,
					TopicName: []byte(target_topic),
					Message:   []byte(fmt.Sprintf("%02f", value)),
				})
				if err != nil {
					panic(err)
				}
			})

			acc.Thermostat.TargetHeatingCoolingState.OnValueRemoteUpdate(func(value int) {
				log.Printf("%s TargetHeatingCoolingState: %d", info.Name, value)
				if value > 1 {
					value = 1
				}
				err = cli.Publish(&client.PublishOptions{
					QoS:       mqtt.QoS0,
					Retain:    true,
					TopicName: []byte(active_topic),
					Message:   []byte(fmt.Sprintf("%d", value)),
				})
				if err != nil {
					panic(err)
				}
			})

			err = cli.Subscribe(&client.SubscribeOptions{
				SubReqs: []*client.SubReq{
					&client.SubReq{
						TopicFilter: []byte(state_topic),
						QoS:         mqtt.QoS0,
						// Define the processing of the message handler.
						Handler: func(topicName, message []byte) {
							new_value, err := strconv.ParseFloat(string(message), 64)
							if err != nil {
								panic(err)
							}
							// log.Printf("%s value externally changed to %f\n", string(topicName), new_value)
							acc.Thermostat.CurrentTemperature.SetValue(new_value)
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}

			err = cli.Subscribe(&client.SubscribeOptions{
				SubReqs: []*client.SubReq{
					&client.SubReq{
						TopicFilter: []byte(target_topic),
						QoS:         mqtt.QoS0,
						// Define the processing of the message handler.
						Handler: func(topicName, message []byte) {
							new_value, err := strconv.ParseFloat(string(message), 64)
							if err != nil {
								panic(err)
							}
							// log.Printf("%s value externally changed to %f\n", string(topicName), new_value)
							acc.Thermostat.TargetTemperature.SetValue(new_value)
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}

			err = cli.Subscribe(&client.SubscribeOptions{
				SubReqs: []*client.SubReq{
					&client.SubReq{
						TopicFilter: []byte(active_topic),
						QoS:         mqtt.QoS0,
						// Define the processing of the message handler.
						Handler: func(topicName, message []byte) {
							new_value, err := strconv.ParseInt(string(message), 10, 0)
							if err != nil {
								panic(err)
							}
							log.Printf("%s value externally changed to %d\n", string(topicName), new_value)
							acc.Thermostat.CurrentHeatingCoolingState.SetValue(int(new_value))
							acc.Thermostat.TargetHeatingCoolingState.SetValue(int(new_value))
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}

			accessories = append(accessories, acc.Accessory)
		} else {
			acc := accessory.NewTemperatureSensor(info, 0, -20, 40, 0.1)

			err = cli.Subscribe(&client.SubscribeOptions{
				SubReqs: []*client.SubReq{
					&client.SubReq{
						TopicFilter: []byte(state_topic),
						QoS:         mqtt.QoS0,
						// Define the processing of the message handler.
						Handler: func(topicName, message []byte) {
							new_value, err := strconv.ParseFloat(string(message), 64)
							if err != nil {
								panic(err)
							}
							// log.Printf("%s value externally changed to %f\n", string(topicName), new_value)
							acc.TempSensor.CurrentTemperature.SetValue(new_value)
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}

			accessories = append(accessories, acc.Accessory)
		}

	}

	transport_config := hc.Config{Pin: config["pin"].(string), StoragePath: config["storage_path"].(string)}
	t, err := hc.NewIPTransport(transport_config, accessories[0], accessories[1:]...)
	if err != nil {
		log.Fatal(err)
	}

	hc.OnTermination(func() {
		t.Stop()
	})

	t.Start()
}
