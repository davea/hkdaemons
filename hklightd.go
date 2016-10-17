package main

import (
	"encoding/json"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/brutella/log"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
	"io/ioutil"
)

func main() {
	log.Verbose = true
	log.Info = true

	file, err := ioutil.ReadFile("./hklightd.json")
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
			Manufacturer: accessory_config["manufacturer"].(string),
		}
		control_topic := fmt.Sprintf("light_%s/control", accessory_config["machine_id"].(string))
		state_topic := fmt.Sprintf("light_%s/state", accessory_config["machine_id"].(string))

		acc := accessory.NewLightbulb(info)

		acc.Lightbulb.On.OnValueRemoteUpdate(func(on bool) {
			value := "off"
			if on {
				value = "on"
			} else {
				value = "off"
			}
			log.Printf("Switching %s", value)
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte(control_topic),
				Message:   []byte(fmt.Sprintf("power:%s", value)),
			})
			if err != nil {
				panic(err)
			}
		})

		acc.Lightbulb.Brightness.OnValueRemoteUpdate(func(value int) {
			log.Printf("Setting brightness to %d", value)
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte(control_topic),
				Message:   []byte(fmt.Sprintf("b:%d", value)),
			})
			if err != nil {
				panic(err)
			}
		})

		acc.Lightbulb.Hue.OnValueRemoteUpdate(func(value float64) {
			log.Printf("Setting hue to %f", value)
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte(control_topic),
				Message:   []byte(fmt.Sprintf("h:%f", value)),
			})
			if err != nil {
				panic(err)
			}
		})

		acc.Lightbulb.Saturation.OnValueRemoteUpdate(func(value float64) {
			log.Printf("Setting saturation to %f", value)
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte(control_topic),
				Message:   []byte(fmt.Sprintf("s:%f", value)),
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
						new_value := "on" == string(message)
						if acc.Lightbulb.On.GetValue() != new_value {
							log.Printf("Value externally changed to %t\n", new_value)
							acc.Lightbulb.On.SetValue(new_value)
						}
					},
				},
			},
		})
		if err != nil {
			panic(err)
		}
		accessories = append(accessories, acc.Accessory)
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
